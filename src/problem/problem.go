package problem

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"justcpc-online-judge/src/store"
)

var ErrForbidden = errors.New("permission denied")

type ResourceLimit struct {
	TimeLimitMS   int64  `json:"time_limit_ms"`
	MemoryLimitMB uint64 `json:"memory_limit_mb"`
}
type Limits map[string]ResourceLimit

type CompareOptions struct {
	FloatingPoint bool    `json:"floating_point"`
	Epsilon       float64 `json:"epsilon"`
}

func DefaultLimits() Limits {
	return Limits{"cpp": {1000, 256}, "java": {1000, 256}, "python3": {2000, 512}}
}

type Problem struct {
	ID                int64          `json:"id"`
	Title             string         `json:"title"`
	StatementMarkdown string         `json:"statement_markdown"`
	Score             int64          `json:"-"`
	ScoreText         string         `json:"score"`
	ResourceLimits    Limits         `json:"resource_limits"`
	Compare           CompareOptions `json:"compare"`
	TestdataPath      string         `json:"testdata_path,omitempty"`
	OwnerAccountID    *int64         `json:"owner_account_id,omitempty"`
	CreatedAt         string         `json:"created_at"`
	UpdatedAt         string         `json:"updated_at"`
}

type Summary struct {
	ID                   int64  `json:"id"`
	Title                string `json:"title"`
	Score                string `json:"score"`
	AcceptingSubmissions bool   `json:"accepting_submissions"`
	CanEdit              bool   `json:"can_edit"`
}

type PublicDetail struct {
	Summary
	ResourceLimits    Limits `json:"resource_limits"`
	StatementMarkdown string `json:"statement_markdown"`
}

type EditorDetail struct {
	ID                int64          `json:"id"`
	Title             string         `json:"title"`
	StatementMarkdown string         `json:"statement_markdown"`
	Score             string         `json:"score"`
	ResourceLimits    Limits         `json:"resource_limits"`
	Compare           CompareOptions `json:"compare"`
	TestdataReady     bool           `json:"testdata_ready"`
	TestCaseCount     int            `json:"test_case_count"`
	CreatedAt         string         `json:"created_at"`
	UpdatedAt         string         `json:"updated_at"`
}

type CreateInput struct {
	Title             string         `json:"title"`
	StatementMarkdown string         `json:"statement_markdown"`
	Score             string         `json:"score"`
	ResourceLimits    Limits         `json:"resource_limits"`
	Compare           CompareOptions `json:"compare"`
}
type UpdateInput = CreateInput
type Service struct{ Store *store.Store }

func (s Service) Create(ctx context.Context, ownerID int64, input CreateInput) (Problem, error) {
	if strings.TrimSpace(input.Title) == "" {
		return Problem{}, errors.New("title is required")
	}
	score, err := store.ParseScore(input.Score)
	if err != nil {
		return Problem{}, err
	}
	limits := mergeLimits(input.ResourceLimits)
	if input.Compare.Epsilon < 0 {
		return Problem{}, errors.New("epsilon must not be negative")
	}
	encoded, _ := json.Marshal(limits)
	compare, _ := json.Marshal(input.Compare)
	now := store.Now()
	result, err := s.Store.DB.ExecContext(ctx, `INSERT INTO problems(title,statement_markdown,owner_account_id,score,resource_limits_json,compare_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, input.Title, input.StatementMarkdown, ownerID, score, string(encoded), string(compare), now, now)
	if err != nil {
		return Problem{}, err
	}
	id, _ := result.LastInsertId()
	return Problem{ID: id, Title: input.Title, StatementMarkdown: input.StatementMarkdown, Score: score, ScoreText: store.FormatScore(score), ResourceLimits: limits, Compare: input.Compare, OwnerAccountID: &ownerID, CreatedAt: now, UpdatedAt: now}, nil
}

func (s Service) Update(ctx context.Context, id, actorID int64, input UpdateInput) (Problem, error) {
	if err := s.canManage(ctx, id, actorID); err != nil {
		return Problem{}, err
	}
	if strings.TrimSpace(input.Title) == "" {
		return Problem{}, errors.New("title is required")
	}
	score, err := store.ParseScore(input.Score)
	if err != nil {
		return Problem{}, err
	}
	limits := mergeLimits(input.ResourceLimits)
	if input.Compare.Epsilon < 0 {
		return Problem{}, errors.New("epsilon must not be negative")
	}
	encoded, _ := json.Marshal(limits)
	compare, _ := json.Marshal(input.Compare)
	now := store.Now()
	if _, err = s.Store.DB.ExecContext(ctx, `UPDATE problems SET title=?,statement_markdown=?,score=?,resource_limits_json=?,compare_json=?,updated_at=? WHERE id=?`, input.Title, input.StatementMarkdown, score, string(encoded), string(compare), now, id); err != nil {
		return Problem{}, err
	}
	return s.Get(ctx, id)
}

func (s Service) ListPublic(ctx context.Context, actorID int64) ([]Summary, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT p.id,COALESCE(p.title,''),p.score,
		CASE WHEN COALESCE(p.testdata_path,'')<>'' THEN 1 ELSE 0 END,
		CASE WHEN p.owner_account_id=? THEN 1 ELSE 0 END
		FROM problems p ORDER BY p.id DESC`, actorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Summary
	for rows.Next() {
		var item Summary
		var score int64
		var accepting, editable int
		if err := rows.Scan(&item.ID, &item.Title, &score, &accepting, &editable); err != nil {
			return nil, err
		}
		item.AcceptingSubmissions = accepting != 0
		item.CanEdit = editable != 0
		item.Score = store.FormatScore(score)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s Service) Public(ctx context.Context, id, actorID int64) (PublicDetail, error) {
	var item PublicDetail
	var limits string
	var score int64
	var accepting, editable int
	err := s.Store.DB.QueryRowContext(ctx, `SELECT p.id,COALESCE(p.title,''),p.score,p.resource_limits_json,
		CASE WHEN COALESCE(p.testdata_path,'')<>'' THEN 1 ELSE 0 END,
		CASE WHEN p.owner_account_id=? THEN 1 ELSE 0 END,COALESCE(p.statement_markdown,'')
		FROM problems p WHERE p.id=?`, actorID, id).Scan(
		&item.ID, &item.Title, &score, &limits, &accepting, &editable, &item.StatementMarkdown,
	)
	if err != nil {
		return item, err
	}
	item.AcceptingSubmissions = accepting != 0
	item.CanEdit = editable != 0
	item.Score = store.FormatScore(score)
	_ = json.Unmarshal([]byte(limits), &item.ResourceLimits)
	item.ResourceLimits = mergeLimits(item.ResourceLimits)
	return item, nil
}

func (s Service) Editor(ctx context.Context, id, actorID int64) (EditorDetail, error) {
	if err := s.canManage(ctx, id, actorID); err != nil {
		return EditorDetail{}, err
	}
	p, err := s.Get(ctx, id)
	if err != nil {
		return EditorDetail{}, err
	}
	item := EditorDetail{ID: p.ID, Title: p.Title, StatementMarkdown: p.StatementMarkdown, Score: p.ScoreText, ResourceLimits: p.ResourceLimits, Compare: p.Compare, TestdataReady: p.TestdataPath != "", CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
	if !item.TestdataReady {
		return item, nil
	}
	entries, err := os.ReadDir(s.Store.Absolute(p.TestdataPath))
	if err != nil {
		return EditorDetail{}, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".in") {
			item.TestCaseCount++
		}
	}
	return item, nil
}

func (s Service) Get(ctx context.Context, id int64) (Problem, error) {
	return scan(s.Store.DB.QueryRowContext(ctx, `SELECT id,title,statement_markdown,score,resource_limits_json,COALESCE(compare_json,'{}'),COALESCE(testdata_path,''),owner_account_id,created_at,updated_at FROM problems WHERE id=?`, id))
}

type scanner interface{ Scan(...any) error }

func scan(row scanner) (Problem, error) {
	var p Problem
	var limits string
	var compare string
	var owner sql.NullInt64
	var title, statement sql.NullString
	err := row.Scan(&p.ID, &title, &statement, &p.Score, &limits, &compare, &p.TestdataPath, &owner, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	if owner.Valid {
		p.OwnerAccountID = &owner.Int64
	}
	p.Title = title.String
	p.StatementMarkdown = statement.String
	p.ScoreText = store.FormatScore(p.Score)
	_ = json.Unmarshal([]byte(limits), &p.ResourceLimits)
	_ = json.Unmarshal([]byte(compare), &p.Compare)
	p.ResourceLimits = mergeLimits(p.ResourceLimits)
	return p, nil
}

func (s Service) UploadTestdata(ctx context.Context, id, actorID int64, archive []byte) error {
	if err := s.canManage(ctx, id, actorID); err != nil {
		return err
	}
	if len(archive) == 0 {
		return errors.New("test data package is empty")
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("read ZIP: %w", err)
	}
	stage, err := os.MkdirTemp(s.Store.Absolute("tests"), ".stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	seen := map[int]map[string]bool{}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		number, kind, ok := testName(file.Name)
		if !ok {
			return fmt.Errorf("invalid test file %q", file.Name)
		}
		if seen[number] == nil {
			seen[number] = map[string]bool{}
		}
		if seen[number][kind] {
			return fmt.Errorf("duplicate test file %q", file.Name)
		}
		seen[number][kind] = true
		input, err := file.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(input, 1<<30))
		input.Close()
		if err != nil {
			return err
		}
		// 无输入题以及预期无输出的测试点允许空文件；配对和连续编号仍在下方校验。
		if err := os.WriteFile(filepath.Join(stage, file.Name), data, 0600); err != nil {
			return err
		}
	}
	if len(seen) == 0 {
		return errors.New("no test files found")
	}
	for i := 1; i <= len(seen); i++ {
		pair := seen[i]
		if pair == nil || !pair["in"] || !pair["out"] {
			return fmt.Errorf("test point %d must contain %d.in and %d.out", i, i, i)
		}
	}
	relative := filepath.Join("tests", strconv.FormatInt(id, 10))
	destination := s.Store.Absolute(relative)
	backup := destination + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(destination); err == nil {
		if err := os.Rename(destination, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(stage, destination); err != nil {
		_ = os.Rename(backup, destination)
		return err
	}
	_ = os.RemoveAll(backup)
	_, err = s.Store.DB.ExecContext(ctx, `UPDATE problems SET testdata_path=?,updated_at=? WHERE id=?`, relative, store.Now(), id)
	return err
}

func (s Service) canManage(ctx context.Context, id, actorID int64) error {
	var owner sql.NullInt64
	err := s.Store.DB.QueryRowContext(ctx, `SELECT owner_account_id FROM problems WHERE id=?`, id).Scan(&owner)
	if err != nil {
		return err
	}
	if !owner.Valid || owner.Int64 != actorID {
		return ErrForbidden
	}
	return nil
}

// Delete 删除题目：管理员可删除任意题目，普通用户仅能删除自己创建的题目。
// 提交记录与比赛关联由外键级联清理；测试数据、提交源码与评测输出等磁盘文件在此一并删除。
func (s Service) Delete(ctx context.Context, id, actorID int64, isAdmin bool) error {
	p, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if !isAdmin && (p.OwnerAccountID == nil || *p.OwnerAccountID != actorID) {
		return ErrForbidden
	}
	submissionIDs, err := s.submissionIDs(ctx, id)
	if err != nil {
		return err
	}
	if _, err := s.Store.DB.ExecContext(ctx, `DELETE FROM problems WHERE id=?`, id); err != nil {
		return err
	}
	if p.TestdataPath != "" {
		_ = os.RemoveAll(s.Store.Absolute(p.TestdataPath))
	}
	for _, submissionID := range submissionIDs {
		name := strconv.FormatInt(submissionID, 10)
		_ = os.RemoveAll(s.Store.Absolute(filepath.Join("submissions", name)))
		_ = os.RemoveAll(s.Store.Absolute(filepath.Join("outputs", name)))
	}
	return nil
}

func (s Service) submissionIDs(ctx context.Context, problemID int64) ([]int64, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT id FROM submissions WHERE problem_id=?`, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func mergeLimits(overrides Limits) Limits {
	defaults := DefaultLimits()
	for language, value := range overrides {
		if _, ok := defaults[language]; ok {
			if value.TimeLimitMS > 0 {
				defaults[language] = ResourceLimit{TimeLimitMS: value.TimeLimitMS, MemoryLimitMB: defaults[language].MemoryLimitMB}
			}
			if value.MemoryLimitMB > 0 {
				entry := defaults[language]
				entry.MemoryLimitMB = value.MemoryLimitMB
				defaults[language] = entry
			}
		}
	}
	return defaults
}
func testName(name string) (int, string, bool) {
	if strings.Contains(name, "/") {
		return 0, "", false
	}
	parts := strings.Split(name, ".")
	if len(parts) != 2 || (parts[1] != "in" && parts[1] != "out") {
		return 0, "", false
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n < 1 || strconv.Itoa(n) != parts[0] {
		return 0, "", false
	}
	return n, parts[1], true
}
func (s Service) Cases(p Problem) ([]map[string]string, error) {
	if p.TestdataPath == "" {
		return nil, errors.New("problem has no test data")
	}
	entries, err := os.ReadDir(s.Store.Absolute(p.TestdataPath))
	if err != nil {
		return nil, err
	}
	var numbers []int
	for _, entry := range entries {
		if n, kind, ok := testName(entry.Name()); ok && kind == "in" {
			numbers = append(numbers, n)
		}
	}
	sort.Ints(numbers)
	cases := make([]map[string]string, 0, len(numbers))
	for _, n := range numbers {
		cases = append(cases, map[string]string{"id": strconv.Itoa(n), "input_path": s.Store.Absolute(filepath.Join(p.TestdataPath, fmt.Sprintf("%d.in", n))), "expected_path": s.Store.Absolute(filepath.Join(p.TestdataPath, fmt.Sprintf("%d.out", n)))})
	}
	return cases, nil
}
