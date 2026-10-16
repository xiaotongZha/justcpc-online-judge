package judge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"justcpc-online-judge/src/problem"
	"justcpc-online-judge/src/store"
	"justcpc-online-judge/src/submission"
)

const (
	leaseDuration    = 15 * time.Minute
	pollInterval     = 500 * time.Millisecond
	recoveryInterval = 30 * time.Second
)

type Dispatcher struct {
	Store       *store.Store
	Submissions submission.Service
	Problems    problem.Service
	Client      Client
	Workers     int
}

type queueTask struct {
	ID           int64
	SubmissionID int64
}

type QueueStatus struct {
	ID           int64  `json:"id"`
	SubmissionID int64  `json:"submission_id"`
	State        string `json:"state"`
	Attempt      int    `json:"attempt"`
	Error        string `json:"error,omitempty"`
}

func (d *Dispatcher) QueueStatus(ctx context.Context) ([]QueueStatus, error) {
	rows, err := d.Store.DB.QueryContext(ctx, `SELECT id,submission_id,state,attempt,COALESCE(error,'') FROM judge_queue ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]QueueStatus, 0)
	for rows.Next() {
		var item QueueStatus
		if err := rows.Scan(&item.ID, &item.SubmissionID, &item.State, &item.Attempt, &item.Error); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (d *Dispatcher) RetryFailed(ctx context.Context) (int64, error) {
	result, err := d.Store.DB.ExecContext(ctx, `UPDATE judge_queue SET state='queued',lease_until=NULL,started_at=NULL,error=NULL WHERE state='failed'`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *Dispatcher) Run(ctx context.Context) error {
	if d.Workers < 0 {
		return errors.New("judge workers must be non-negative (0 means automatic)")
	}
	if err := d.recoverExpired(ctx); err != nil {
		return err
	}
	// A normal submission writes its source before its queue row. If the process
	// stops in that narrow interval, restore the durable task at next startup.
	// Contest submissions deliberately have no account_id and remain pending
	// until an administrator selects them for a batch evaluation.
	if _, err := d.Store.DB.ExecContext(ctx, `INSERT INTO judge_queue(submission_id,state,queued_at)
		SELECT s.id,'queued',? FROM submissions s
		LEFT JOIN judge_queue q ON q.submission_id=s.id
		WHERE s.account_id IS NOT NULL AND s.status='waiting' AND q.id IS NULL`, store.Now()); err != nil {
		return fmt.Errorf("recover normal submission tasks: %w", err)
	}
	workerCount, err := d.resolveWorkers(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() { defer workers.Done(); d.worker(ctx) }()
	}
	defer workers.Wait()
	ticker := time.NewTicker(recoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := d.recoverExpired(ctx); err != nil {
				log.Printf("%v", err)
			}
		}
	}
}

func (d *Dispatcher) resolveWorkers(ctx context.Context) (int, error) {
	if d.Workers > 0 {
		log.Printf("judge dispatcher capacity: mode=manual workers=%d", d.Workers)
		return d.Workers, nil
	}
	for {
		workers, err := d.Client.Capacity(ctx)
		if err == nil {
			log.Printf("judge dispatcher capacity: mode=auto workers=%d (reported by engine)", workers)
			return workers, nil
		}
		if errors.Is(err, ErrCapacityUnsupported) {
			log.Printf("judge dispatcher capacity: old engine has no capacity endpoint; using 1 worker")
			return 1, nil
		}
		log.Printf("waiting for judge engine capacity; submissions remain queued: %v", err)
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, ctx.Err()
		case <-timer.C:
		}
	}
}

func (d *Dispatcher) recoverExpired(ctx context.Context) error {
	if _, err := d.Store.DB.ExecContext(ctx, `UPDATE judge_queue SET state='queued',lease_until=NULL,started_at=NULL WHERE state='running' AND lease_until <= ?`, store.Now()); err != nil {
		return fmt.Errorf("recover expired judge tasks: %w", err)
	}
	return nil
}

func (d *Dispatcher) worker(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		task, err := d.claim(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				continue
			}
		}
		if task == nil {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				continue
			}
		}
		d.execute(ctx, *task)
	}
}

func (d *Dispatcher) claim(ctx context.Context) (*queueTask, error) {
	return store.WithTx(ctx, d.Store.DB, func(tx *sql.Tx) (*queueTask, error) {
		row := tx.QueryRowContext(ctx, `SELECT id,submission_id FROM judge_queue WHERE state='queued' ORDER BY queued_at,id LIMIT 1`)
		var task queueTask
		if err := row.Scan(&task.ID, &task.SubmissionID); errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		result, err := tx.ExecContext(ctx, `UPDATE judge_queue SET state='running',attempt=attempt+1,started_at=?,lease_until=? WHERE id=? AND state='queued'`, store.Now(), store.NowAfter(leaseDuration), task.ID)
		if err != nil {
			return nil, err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return nil, nil
		}
		return &task, nil
	})
}

func (d *Dispatcher) execute(ctx context.Context, task queueTask) {
	done := make(chan struct{})
	go d.renewLease(ctx, task.ID, done)
	defer close(done)
	sub, p, source, err := d.Submissions.LoadExecution(ctx, task.SubmissionID)
	if err != nil {
		d.fail(ctx, task.ID, err)
		return
	}
	cases, err := d.Problems.Cases(p)
	if err != nil {
		d.fail(ctx, task.ID, err)
		return
	}
	limits := make(map[string]ResourceLimit, len(p.ResourceLimits))
	for language, limit := range p.ResourceLimits {
		limits[language] = ResourceLimit{TimeLimitMS: limit.TimeLimitMS, MemoryLimitMB: limit.MemoryLimitMB}
	}
	engineCases := make([]Case, 0, len(cases))
	for _, item := range cases {
		engineCases = append(engineCases, Case{ID: item["id"], InputPath: item["input_path"], ExpectedPath: item["expected_path"]})
	}
	outputRelative := filepath.Join("outputs", fmt.Sprint(task.SubmissionID))
	response, busy, err := d.Client.Execute(ctx, Task{ProblemID: fmt.Sprint(p.ID), Language: sub.Language, Source: source, Problem: Problem{ID: fmt.Sprint(p.ID), Limits: limits, Compare: CompareOption{FloatingPoint: p.Compare.FloatingPoint, Epsilon: p.Compare.Epsilon}, Cases: engineCases}, OutputDir: d.Store.Absolute(outputRelative)})
	if busy {
		d.release(ctx, task.ID)
		// 满载时先等待一个轮询周期，避免立即抢回同一任务并密集请求引擎。
		select {
		case <-ctx.Done():
		case <-time.After(pollInterval):
		}
		return
	}
	if err != nil {
		d.fail(ctx, task.ID, err)
		return
	}
	if response.Verdict == "SYS" {
		d.fail(ctx, task.ID, errors.New(systemErrorMessage(response)))
		return
	}
	status, score, result := normalize(response, p.Score, outputRelative)
	encoded, err := json.Marshal(result)
	if err != nil {
		d.fail(ctx, task.ID, err)
		return
	}
	_, err = store.WithTx(ctx, d.Store.DB, func(tx *sql.Tx) (struct{}, error) {
		if _, err := tx.ExecContext(ctx, `UPDATE submissions SET status=?,score=?,judge_result_json=?,judged_at=? WHERE id=?`, status, score, string(encoded), store.Now(), task.SubmissionID); err != nil {
			return struct{}{}, err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM judge_queue WHERE id=?`, task.ID)
		return struct{}{}, err
	})
	if err != nil {
		d.fail(ctx, task.ID, err)
	}
}

func (d *Dispatcher) renewLease(ctx context.Context, id int64, done <-chan struct{}) {
	ticker := time.NewTicker(leaseDuration / 3)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = d.Store.DB.ExecContext(context.Background(), `UPDATE judge_queue SET lease_until=? WHERE id=? AND state='running'`, store.NowAfter(leaseDuration), id)
		}
	}
}

func (d *Dispatcher) release(ctx context.Context, id int64) {
	_, _ = d.Store.DB.ExecContext(ctx, `UPDATE judge_queue SET state='queued',lease_until=NULL,started_at=NULL WHERE id=?`, id)
}
func (d *Dispatcher) fail(ctx context.Context, id int64, err error) {
	_, _ = d.Store.DB.ExecContext(ctx, `UPDATE judge_queue SET state='failed',lease_until=NULL,error=? WHERE id=?`, truncate(err.Error(), 4096), id)
}

type judgeResult struct {
	CompileLog string       `json:"compile_log,omitempty"`
	Cases      []CaseResult `json:"cases"`
}

func normalize(response Response, totalScore int64, outputRelative string) (string, int64, judgeResult) {
	result := judgeResult{CompileLog: strings.TrimSpace(response.Compile.Stderr + "\n" + response.Compile.Error), Cases: response.Cases}
	if response.Verdict == "CE" {
		return "CE", 0, result
	}
	passed := 0
	for index := range result.Cases {
		if result.Cases[index].Verdict == "AC" {
			passed++
		}
		if result.Cases[index].OutputPath != "" {
			result.Cases[index].OutputPath = filepath.Join(outputRelative, filepath.Base(result.Cases[index].OutputPath))
		}
	}
	if len(result.Cases) == 0 {
		return "WA", 0, result
	}
	score := (totalScore*int64(passed) + int64(len(result.Cases))/2) / int64(len(result.Cases))
	if passed == len(result.Cases) {
		return "AC", totalScore, result
	}
	return "WA", score, result
}

func truncate(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	return value[:maximum]
}

func systemErrorMessage(response Response) string {
	parts := []string{"judge engine system error"}
	if response.Compile.Error != "" {
		parts = append(parts, response.Compile.Error)
	}
	for _, result := range response.Cases {
		if result.Error != "" {
			parts = append(parts, "case "+result.ID+": "+result.Error)
		}
	}
	return strings.Join(parts, "; ")
}
