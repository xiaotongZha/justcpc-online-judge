package contest

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/csv"
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

type Service struct{ Store *store.Store }

// Contest 是一组既有题目的有序捆绑。考试时间由机房考试平台设定，OJ 不负责。
type Contest struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	StatementPDFPath string `json:"statement_pdf_path,omitempty"`
	ProblemCount     int    `json:"problem_count"`
	ContestantCount  int    `json:"contestant_count"`
	SubmissionCount  int    `json:"submission_count"`
	CreatedAt        string `json:"created_at"`
}

// ContestProblem 是某题在某个比赛中的引用与分值（分值是比赛的属性）。
type ContestProblem struct {
	ProblemID      int64           `json:"problem_id"`
	Label          string          `json:"label"`
	Title          string          `json:"title"`
	Score          string          `json:"score"`
	OrderIndex     int             `json:"order_index"`
	ResourceLimits json.RawMessage `json:"resource_limits"`
}

type Contestant struct {
	ID        int64  `json:"id"`
	StudentID string `json:"student_id"`
	Name      string `json:"name"`
}

type ContestDetail struct {
	Contest
	Problems    []ContestProblem `json:"problems"`
	Contestants []Contestant     `json:"contestants"`
}

type CreateProblem struct {
	ProblemID int64  `json:"problem_id"`
	Score     string `json:"score"`
}

type CreateInput struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Problems    []CreateProblem `json:"problems"`
}

// Create 用既有题目创建一场比赛。名单提供初始密码，仅写入导出文件，不落库。
func (s Service) Create(ctx context.Context, actorID int64, input CreateInput,
	statementPDF []byte, rosterFileName string, rosterData []byte) (Contest, error) {

	if strings.TrimSpace(input.Name) == "" {
		return Contest{}, errors.New("比赛名称不能为空")
	}
	if len(input.Problems) == 0 {
		return Contest{}, errors.New("至少选择一道题目")
	}
	if len(input.Problems) > 26 {
		return Contest{}, errors.New("最多 26 道题目（A-Z）")
	}
	if len(statementPDF) == 0 {
		return Contest{}, errors.New("必须上传比赛题面 PDF")
	}

	entries, err := ParseRoster(rosterFileName, rosterData)
	if err != nil {
		return Contest{}, err
	}

	type prepared struct {
		problemID  int64
		label      string
		scoreCents int64
	}
	ordered := make([]prepared, 0, len(input.Problems))
	seen := map[int64]bool{}
	for index, item := range input.Problems {
		score, err := store.ParseScore(item.Score)
		if err != nil {
			return Contest{}, fmt.Errorf("第 %d 题分值无效：%w", index+1, err)
		}
		if item.ProblemID <= 0 || seen[item.ProblemID] {
			return Contest{}, fmt.Errorf("第 %d 题无效或重复", index+1)
		}
		var exists int
		if err := s.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM problems WHERE id=?`, item.ProblemID).Scan(&exists); err != nil {
			return Contest{}, err
		}
		if exists == 0 {
			return Contest{}, fmt.Errorf("题目 #%d 不存在", item.ProblemID)
		}
		seen[item.ProblemID] = true
		ordered = append(ordered, prepared{problemID: item.ProblemID, label: string(rune('A' + index)), scoreCents: score})
	}

	now := store.Now()
	var contestID int64
	_, err = store.WithTx(ctx, s.Store.DB, func(tx *sql.Tx) (struct{}, error) {
		result, err := tx.ExecContext(ctx,
			`INSERT INTO contests(name,description,created_at) VALUES(?,?,?)`,
			input.Name, input.Description, now)
		if err != nil {
			return struct{}{}, err
		}
		contestID, _ = result.LastInsertId()
		for index, item := range ordered {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO contest_problems(contest_id,problem_id,label,order_index,score_cents) VALUES(?,?,?,?,?)`,
				contestID, item.problemID, item.label, index, item.scoreCents); err != nil {
				return struct{}{}, err
			}
		}
		for _, entry := range entries {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO contestants(contest_id,student_id,name,created_at) VALUES(?,?,?,?)`,
				contestID, entry.StudentID, entry.Name, now); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, auditTx(ctx, tx, actorID, "contest.create", "contest", strconv.FormatInt(contestID, 10), map[string]any{
			"name": input.Name, "problems": len(ordered), "contestants": len(entries),
		})
	})
	if err != nil {
		return Contest{}, err
	}

	statementPath := filepath.Join("contest-statements", strconv.FormatInt(contestID, 10), "statement.pdf")
	if err := s.Store.WriteFile(statementPath, statementPDF); err != nil {
		return Contest{}, err
	}
	rosterPath := filepath.Join("contest-rosters", strconv.FormatInt(contestID, 10), "contestants.csv")
	if err := s.Store.WriteFile(rosterPath, RosterCSV(entries)); err != nil {
		return Contest{}, err
	}
	if _, err := s.Store.DB.ExecContext(ctx, `UPDATE contests SET statement_pdf_path=?,roster_path=? WHERE id=?`, statementPath, rosterPath, contestID); err != nil {
		return Contest{}, err
	}
	return s.Get(ctx, contestID)
}

func (s Service) Get(ctx context.Context, contestID int64) (Contest, error) {
	var item Contest
	var statement sql.NullString
	err := s.Store.DB.QueryRowContext(ctx,
		`SELECT c.id,c.name,c.description,COALESCE(c.statement_pdf_path,''),c.created_at,
			(SELECT COUNT(*) FROM contest_problems cp WHERE cp.contest_id=c.id),
			(SELECT COUNT(*) FROM contestants t WHERE t.contest_id=c.id),
			(SELECT COUNT(*) FROM submissions s JOIN contestants t ON t.id=s.contestant_id WHERE t.contest_id=c.id)
		 FROM contests c WHERE c.id=?`, contestID).
		Scan(&item.ID, &item.Name, &item.Description, &statement, &item.CreatedAt,
			&item.ProblemCount, &item.ContestantCount, &item.SubmissionCount)
	if err != nil {
		return item, err
	}
	item.StatementPDFPath = statement.String
	return item, nil
}

func (s Service) List(ctx context.Context) ([]Contest, error) {
	rows, err := s.Store.DB.QueryContext(ctx,
		`SELECT c.id,c.name,c.description,COALESCE(c.statement_pdf_path,''),c.created_at,
			(SELECT COUNT(*) FROM contest_problems cp WHERE cp.contest_id=c.id),
			(SELECT COUNT(*) FROM contestants t WHERE t.contest_id=c.id),
			(SELECT COUNT(*) FROM submissions s JOIN contestants t ON t.id=s.contestant_id WHERE t.contest_id=c.id)
		 FROM contests c ORDER BY c.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Contest, 0)
	for rows.Next() {
		var item Contest
		var statement sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &statement, &item.CreatedAt,
			&item.ProblemCount, &item.ContestantCount, &item.SubmissionCount); err != nil {
			return nil, err
		}
		item.StatementPDFPath = statement.String
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s Service) Detail(ctx context.Context, contestID int64) (ContestDetail, error) {
	base, err := s.Get(ctx, contestID)
	if err != nil {
		return ContestDetail{}, err
	}
	detail := ContestDetail{Contest: base}
	rows, err := s.Store.DB.QueryContext(ctx,
		`SELECT cp.problem_id,cp.label,COALESCE(p.title,''),cp.score_cents,cp.order_index,COALESCE(p.resource_limits_json,'{}')
		 FROM contest_problems cp JOIN problems p ON p.id=cp.problem_id WHERE cp.contest_id=? ORDER BY cp.order_index`, contestID)
	if err != nil {
		return detail, err
	}
	for rows.Next() {
		var item ContestProblem
		var scoreCents int64
		var limits string
		if err := rows.Scan(&item.ProblemID, &item.Label, &item.Title, &scoreCents, &item.OrderIndex, &limits); err != nil {
			rows.Close()
			return detail, err
		}
		item.Score = store.FormatScore(scoreCents)
		item.ResourceLimits = json.RawMessage(limits)
		detail.Problems = append(detail.Problems, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return detail, err
	}
	crows, err := s.Store.DB.QueryContext(ctx, `SELECT id,student_id,name FROM contestants WHERE contest_id=? ORDER BY student_id`, contestID)
	if err != nil {
		return detail, err
	}
	defer crows.Close()
	for crows.Next() {
		var item Contestant
		if err := crows.Scan(&item.ID, &item.StudentID, &item.Name); err != nil {
			return detail, err
		}
		detail.Contestants = append(detail.Contestants, item)
	}
	return detail, crows.Err()
}

// Delete 删除比赛：一并删除该比赛的考生及其提交记录（含源码/输出文件）、
// 题目关联与成绩单；被引用的题目本身保留。submissions.contestant_id 无级联，
// 因此先删提交再删比赛（其余关联由外键级联清理）。
func (s Service) Delete(ctx context.Context, actorID, contestID int64) error {
	base, err := s.Get(ctx, contestID)
	if err != nil {
		return err
	}
	rows, err := s.Store.DB.QueryContext(ctx,
		`SELECT s.id FROM submissions s JOIN contestants t ON t.id=s.contestant_id WHERE t.contest_id=?`, contestID)
	if err != nil {
		return err
	}
	var submissionIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		submissionIDs = append(submissionIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = store.WithTx(ctx, s.Store.DB, func(tx *sql.Tx) (struct{}, error) {
		if _, err := tx.ExecContext(ctx, `DELETE FROM submissions WHERE contestant_id IN (SELECT id FROM contestants WHERE contest_id=?)`, contestID); err != nil {
			return struct{}{}, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM contests WHERE id=?`, contestID); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, auditTx(ctx, tx, actorID, "contest.delete", "contest", strconv.FormatInt(contestID, 10), map[string]any{"name": base.Name})
	})
	if err != nil {
		return err
	}
	_ = os.RemoveAll(s.Store.Absolute(filepath.Join("contest-statements", strconv.FormatInt(contestID, 10))))
	_ = os.RemoveAll(s.Store.Absolute(filepath.Join("contest-rosters", strconv.FormatInt(contestID, 10))))
	for _, submissionID := range submissionIDs {
		name := strconv.FormatInt(submissionID, 10)
		_ = os.RemoveAll(s.Store.Absolute(filepath.Join("submissions", name)))
		_ = os.RemoveAll(s.Store.Absolute(filepath.Join("outputs", name)))
	}
	return nil
}

func (s Service) StatementPDF(ctx context.Context, contestID int64) (string, error) {
	var path sql.NullString
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT statement_pdf_path FROM contests WHERE id=?`, contestID).Scan(&path); err != nil {
		return "", err
	}
	if !path.Valid || path.String == "" {
		return "", errors.New("比赛没有题面 PDF")
	}
	return s.Store.Absolute(path.String), nil
}

// ---------- 导出考试包 ----------

type examManifest struct {
	PackageType string        `json:"package_type"`
	ContestID   int64         `json:"contest_id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Problems    []examProblem `json:"problems"`
}

type examProblem struct {
	ProblemID      int64           `json:"problem_id"`
	Label          string          `json:"label"`
	Title          string          `json:"title"`
	Score          string          `json:"score"`
	ResourceLimits json.RawMessage `json:"resource_limits"`
}

// ExportExamPackage 生成供机房考试系统导入的考试包：contest.json + statement.pdf + contestants.csv。
func (s Service) ExportExamPackage(ctx context.Context, actorID int64, contestID int64) (string, []byte, error) {
	base, err := s.Get(ctx, contestID)
	if err != nil {
		return "", nil, err
	}
	if base.ProblemCount == 0 {
		return "", nil, errors.New("比赛没有题目")
	}
	detail, err := s.Detail(ctx, contestID)
	if err != nil {
		return "", nil, err
	}
	manifest := examManifest{
		PackageType: "exam", ContestID: base.ID, Name: base.Name, Description: base.Description,
	}
	for _, item := range detail.Problems {
		limits := item.ResourceLimits
		if len(limits) == 0 {
			limits = json.RawMessage(`{}`)
		}
		manifest.Problems = append(manifest.Problems, examProblem{
			ProblemID: item.ProblemID, Label: item.Label, Title: item.Title, Score: item.Score, ResourceLimits: limits,
		})
	}
	manifestData, _ := json.Marshal(manifest)

	var rosterPath sql.NullString
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT COALESCE(roster_path,'') FROM contests WHERE id=?`, contestID).Scan(&rosterPath); err != nil {
		return "", nil, err
	}
	if rosterPath.String == "" {
		return "", nil, errors.New("比赛缺少名单")
	}
	roster, err := os.ReadFile(s.Store.Absolute(rosterPath.String))
	if err != nil {
		return "", nil, fmt.Errorf("读取名单失败：%w", err)
	}
	if _, err := ParseRoster("roster.csv", roster); err != nil {
		return "", nil, fmt.Errorf("比赛名单需要补齐密码后才能导出：%w", err)
	}
	statement, err := os.ReadFile(s.Store.Absolute(base.StatementPDFPath))
	if err != nil {
		return "", nil, fmt.Errorf("读取题面失败：%w", err)
	}

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	write := func(name string, value []byte) error {
		file, err := archive.Create(name)
		if err != nil {
			return err
		}
		_, err = file.Write(value)
		return err
	}
	if err := write("contest.json", manifestData); err != nil {
		return "", nil, err
	}
	if err := write("statement.pdf", statement); err != nil {
		return "", nil, err
	}
	if err := write("contestants.csv", roster); err != nil {
		return "", nil, err
	}
	if err := archive.Close(); err != nil {
		return "", nil, err
	}
	fileName := PackageFileName("考试包", base.Name, contestID)
	if err := s.Audit(ctx, actorID, "contest.export_exam", "contest", strconv.FormatInt(contestID, 10), map[string]any{"file": fileName}); err != nil {
		return fileName, buffer.Bytes(), err
	}
	return fileName, buffer.Bytes(), nil
}

// ---------- 导入提交包 ----------

type submissionManifest struct {
	PackageType string `json:"package_type"`
	ContestID   int64  `json:"contest_id"`
	Name        string `json:"name"`
	Problems    []struct {
		ProblemID int64  `json:"problem_id"`
		Label     string `json:"label"`
	} `json:"problems"`
}

type ImportResult struct {
	Contestants int `json:"contestants"`
	Submissions int `json:"submissions"`
}

// ImportSubmissions 导入考试系统导出的提交包，按题目 ID 关联既有题目。
// 遇到不属于本比赛的题目 ID 直接拒绝，绝不新建题目。
func (s Service) ImportSubmissions(ctx context.Context, actorID int64, contestID int64, archive []byte) (ImportResult, error) {
	return s.ImportSubmissionsMapped(ctx, actorID, contestID, archive, ImportMapping{})
}

func (s Service) ImportSubmissionsMapped(ctx context.Context, actorID int64, contestID int64, archive []byte, mapping ImportMapping) (ImportResult, error) {
	data, err := s.readSubmissions(ctx, contestID, archive)
	if err != nil {
		return ImportResult{}, err
	}
	if err := data.preview.validateMapping(mapping); err != nil {
		return ImportResult{}, err
	}
	labelToProblem, rows, sources := data.labelToProblem, data.rows, data.sources

	now := store.Now()
	result := ImportResult{}
	var written []string
	committed := false
	defer func() {
		if !committed {
			for _, path := range written {
				_ = os.Remove(s.Store.Absolute(path))
				_ = os.Remove(filepath.Dir(s.Store.Absolute(path)))
			}
		}
	}()
	_, err = store.WithTx(ctx, s.Store.DB, func(tx *sql.Tx) (struct{}, error) {
		contestantIDs := map[string]int64{}
		for i := 1; i < len(rows); i++ {
			if len(rows[i]) < 2 || strings.TrimSpace(rows[i][0]) == "" {
				continue
			}
			studentID := strings.TrimSpace(rows[i][0])
			name := strings.TrimSpace(rows[i][1])
			var id int64
			err := tx.QueryRowContext(ctx, `SELECT id FROM contestants WHERE contest_id=? AND student_id=?`, contestID, studentID).Scan(&id)
			if errors.Is(err, sql.ErrNoRows) {
				res, err := tx.ExecContext(ctx, `INSERT INTO contestants(contest_id,student_id,name,created_at) VALUES(?,?,?,?)`, contestID, studentID, name, now)
				if err != nil {
					return struct{}{}, err
				}
				id, _ = res.LastInsertId()
				result.Contestants++
			} else if err != nil {
				return struct{}{}, err
			} else {
				if _, err := tx.ExecContext(ctx, `UPDATE contestants SET name=? WHERE id=?`, name, id); err != nil {
					return struct{}{}, err
				}
			}
			contestantIDs[studentID] = id
		}
		for _, source := range sources {
			contestantID, ok := contestantIDs[source.StudentID]
			if !ok {
				return struct{}{}, fmt.Errorf("提交中的学号 %s 不在名单中", source.StudentID)
			}
			problemID := labelToProblem[source.Label]
			res, err := tx.ExecContext(ctx,
				`INSERT INTO submissions(problem_id,contestant_id,language,source_path,status,submitted_at) VALUES(?,?,?,?,?,?)`,
				problemID, contestantID, source.Language, "", "waiting", now)
			if err != nil {
				return struct{}{}, err
			}
			id, _ := res.LastInsertId()
			relative := filepath.Join("submissions", strconv.FormatInt(id, 10), "main-"+rand.Text()+"."+extension(source.Language))
			if _, err := tx.ExecContext(ctx, `UPDATE submissions SET source_path=? WHERE id=?`, relative, id); err != nil {
				return struct{}{}, err
			}
			written = append(written, relative)
			if err := s.Store.WriteFile(relative, source.Data); err != nil {
				return struct{}{}, err
			}
			result.Submissions++
		}
		return struct{}{}, auditTx(ctx, tx, actorID, "contest.import_submissions", "contest", strconv.FormatInt(contestID, 10), map[string]any{
			"contestants": result.Contestants, "submissions": result.Submissions,
			"legacy_mapping": data.preview.RequiresMapping, "problem_mapping": mapping.Problems,
		})
	})
	if err != nil {
		return ImportResult{}, err
	}
	committed = true
	return result, nil
}

func (s Service) labelProblemMap(ctx context.Context, contestID int64) (map[string]int64, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT label,problem_id FROM contest_problems WHERE contest_id=?`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int64{}
	for rows.Next() {
		var label string
		var problemID int64
		if err := rows.Scan(&label, &problemID); err != nil {
			return nil, err
		}
		result[label] = problemID
	}
	return result, rows.Err()
}

func readZipFiles(archive []byte) (map[string][]byte, error) {
	if len(archive) == 0 {
		return nil, errors.New("上传包为空")
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("读取 ZIP 失败：%w", err)
	}
	files := map[string][]byte{}
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name, "/") || strings.Contains(entry.Name, "../") || files[entry.Name] != nil {
			return nil, fmt.Errorf("非法路径 %q", entry.Name)
		}
		input, err := entry.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(input, 1<<30))
		input.Close()
		if err != nil {
			return nil, err
		}
		files[entry.Name] = content
	}
	return files, nil
}

// ---------- 批量评测与成绩单 ----------

func (s Service) Enqueue(ctx context.Context, contestID int64, submissionIDs []int64) (int, error) {
	if len(submissionIDs) == 0 {
		return 0, errors.New("at least one submission is required")
	}
	return store.WithTx(ctx, s.Store.DB, func(tx *sql.Tx) (int, error) {
		count := 0
		for _, id := range submissionIDs {
			var status string
			var contestant sql.NullInt64
			err := tx.QueryRowContext(ctx,
				`SELECT s.status,s.contestant_id FROM submissions s
				 JOIN contest_problems cp ON cp.problem_id=s.problem_id AND cp.contest_id=? WHERE s.id=?`, contestID, id).
				Scan(&status, &contestant)
			if errors.Is(err, sql.ErrNoRows) {
				return 0, errors.New("提交不属于该比赛")
			}
			if err != nil {
				return 0, err
			}
			if !contestant.Valid {
				return 0, errors.New("only virtual contestant submissions can be batch judged")
			}
			target := id
			if status != "waiting" {
				result, err := tx.ExecContext(ctx, `INSERT INTO submissions(problem_id,contestant_id,language,source_path,status,submitted_at) SELECT problem_id,contestant_id,language,source_path,'waiting',? FROM submissions WHERE id=?`, store.Now(), id)
				if err != nil {
					return 0, err
				}
				target, _ = result.LastInsertId()
			}
			result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO judge_queue(submission_id,state,queued_at) VALUES(?,?,?)`, target, "queued", store.Now())
			if err != nil {
				return 0, err
			}
			changed, _ := result.RowsAffected()
			count += int(changed)
		}
		return count, nil
	})
}

type Scoreboard struct {
	ContestID   int64            `json:"contest_id"`
	GeneratedAt string           `json:"generated_at"`
	Problems    []ContestProblem `json:"problems"`
	Rows        []ScoreboardRow  `json:"rows"`
}

type ScoreboardRow struct {
	HasSubmission bool     `json:"has_submission"`
	Rank          int      `json:"rank"`
	HasScore      bool     `json:"has_score"`
	StudentID     string   `json:"student_id"`
	Name          string   `json:"name"`
	Scores        []string `json:"scores"`
	SubmissionIDs []int64  `json:"submission_ids"`
	Total         string   `json:"total"`
}

func (s Service) buildScoreboard(ctx context.Context, contestID int64) (Scoreboard, error) {
	board := Scoreboard{ContestID: contestID, GeneratedAt: store.Now()}
	type problemInfo struct {
		id         int64
		label      string
		title      string
		scoreCents int64
	}
	rows, err := s.Store.DB.QueryContext(ctx,
		`SELECT cp.problem_id,cp.label,COALESCE(p.title,''),cp.score_cents FROM contest_problems cp
		 JOIN problems p ON p.id=cp.problem_id WHERE cp.contest_id=? ORDER BY cp.order_index`, contestID)
	if err != nil {
		return board, err
	}
	var problems []problemInfo
	for rows.Next() {
		var item problemInfo
		if err := rows.Scan(&item.id, &item.label, &item.title, &item.scoreCents); err != nil {
			rows.Close()
			return board, err
		}
		problems = append(problems, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return board, err
	}
	for _, item := range problems {
		board.Problems = append(board.Problems, ContestProblem{ProblemID: item.id, Label: item.label, Title: item.title, Score: store.FormatScore(item.scoreCents)})
	}

	crows, err := s.Store.DB.QueryContext(ctx, `SELECT id,student_id,name FROM contestants WHERE contest_id=? ORDER BY student_id`, contestID)
	if err != nil {
		return board, err
	}
	defer crows.Close()
	for crows.Next() {
		var contestantID int64
		var row ScoreboardRow
		if err := crows.Scan(&contestantID, &row.StudentID, &row.Name); err != nil {
			return board, err
		}
		var total int64
		hasScore := false
		for _, problem := range problems {
			var score sql.NullInt64
			var submissionID sql.NullInt64
			err := s.Store.DB.QueryRowContext(ctx,
				`SELECT id,score FROM submissions WHERE contestant_id=? AND problem_id=? ORDER BY submitted_at DESC,id DESC LIMIT 1`,
				contestantID, problem.id).Scan(&submissionID, &score)
			if errors.Is(err, sql.ErrNoRows) {
				row.Scores = append(row.Scores, "")
				row.SubmissionIDs = append(row.SubmissionIDs, 0)
				continue
			}
			if err != nil {
				return board, err
			}
			row.HasSubmission = true
			row.SubmissionIDs = append(row.SubmissionIDs, submissionID.Int64)
			if score.Valid {
				row.Scores = append(row.Scores, store.FormatScore(score.Int64))
				total += score.Int64
				hasScore = true
			} else {
				row.Scores = append(row.Scores, "")
			}
		}
		row.HasScore = hasScore
		if hasScore {
			row.Total = store.FormatScore(total)
		} else {
			row.Total = "-"
		}
		board.Rows = append(board.Rows, row)
	}
	sort.SliceStable(board.Rows, func(i, j int) bool {
		if board.Rows[i].HasSubmission != board.Rows[j].HasSubmission {
			return board.Rows[i].HasSubmission
		}
		if board.Rows[i].HasScore != board.Rows[j].HasScore {
			return board.Rows[i].HasScore
		}
		if board.Rows[i].HasScore {
			ai, _ := store.ParseScore(board.Rows[i].Total)
			aj, _ := store.ParseScore(board.Rows[j].Total)
			if ai != aj {
				return ai > aj
			}
		}
		return board.Rows[i].StudentID < board.Rows[j].StudentID
	})
	last := ""
	rank := 0
	for i := range board.Rows {
		if !board.Rows[i].HasScore {
			board.Rows[i].Rank = 0
			continue
		}
		if board.Rows[i].Total != last {
			rank = i + 1
			last = board.Rows[i].Total
		}
		board.Rows[i].Rank = rank
	}
	return board, crows.Err()
}

// ResultPackage 依据当前实时成绩单导出结果包。
func (s Service) ResultPackage(ctx context.Context, contestID int64) ([]byte, error) {
	board, err := s.buildScoreboard(ctx, contestID)
	if err != nil {
		return nil, err
	}
	var contestName string
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT name FROM contests WHERE id=?`, contestID).Scan(&contestName); err != nil {
		return nil, err
	}
	var content bytes.Buffer
	archive := zip.NewWriter(&content)
	write := func(name string, value []byte) error {
		file, err := archive.Create(name)
		if err != nil {
			return err
		}
		_, err = file.Write(value)
		return err
	}
	manifest := struct {
		ContestName           string           `json:"contest_name"`
		ScoreboardGeneratedAt string           `json:"scoreboard_generated_at"`
		Problems              []ContestProblem `json:"problems"`
	}{contestName, board.GeneratedAt, board.Problems}
	manifestData, _ := json.Marshal(manifest)
	if err := write("manifest.json", manifestData); err != nil {
		return nil, err
	}
	if err := write("scoreboard.csv", scoreboardCSV(board)); err != nil {
		return nil, err
	}
	for _, row := range board.Rows {
		for index, submissionID := range row.SubmissionIDs {
			if submissionID == 0 {
				continue
			}
			var sourcePath, language string
			var judgeJSON sql.NullString
			if err := s.Store.DB.QueryRowContext(ctx, `SELECT source_path,language,judge_result_json FROM submissions WHERE id=?`, submissionID).Scan(&sourcePath, &language, &judgeJSON); err != nil {
				return nil, err
			}
			label := board.Problems[index].Label
			base := filepath.Join("submissions", row.StudentID, label)
			source, err := os.ReadFile(s.Store.Absolute(sourcePath))
			if err != nil {
				return nil, err
			}
			if err := write(filepath.Join(base, "source."+extension(language)), source); err != nil {
				return nil, err
			}
			result := map[string]json.RawMessage{}
			if judgeJSON.Valid {
				result["judge_result_json"] = json.RawMessage(judgeJSON.String)
			} else {
				result["judge_result_json"] = json.RawMessage(`{}`)
			}
			resultData, _ := json.Marshal(result)
			if err := write(filepath.Join(base, "result.json"), resultData); err != nil {
				return nil, err
			}
			var details struct {
				Cases []struct {
					ID         string `json:"id"`
					Verdict    string `json:"verdict"`
					OutputPath string `json:"output_path"`
				} `json:"cases"`
			}
			if judgeJSON.Valid && json.Unmarshal([]byte(judgeJSON.String), &details) == nil {
				for _, test := range details.Cases {
					if test.Verdict != "WA" || test.OutputPath == "" || filepath.Base(test.OutputPath) != test.ID+".out" {
						continue
					}
					output, err := os.ReadFile(s.Store.Absolute(test.OutputPath))
					if err != nil {
						return nil, err
					}
					if err := write(filepath.Join(base, "wa-outputs", filepath.Base(test.OutputPath)), output); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

func scoreboardCSV(board Scoreboard) []byte {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xef, 0xbb, 0xbf})
	writer := csv.NewWriter(&buffer)
	header := []string{"排名", "学号", "姓名"}
	for _, item := range board.Problems {
		header = append(header, item.Label)
	}
	_ = writer.Write(append(header, "总分"))
	for _, row := range board.Rows {
		rank := strconv.Itoa(row.Rank)
		if !row.HasScore {
			rank = "-"
		}
		line := []string{rank, row.StudentID, row.Name}
		line = append(line, row.Scores...)
		_ = writer.Write(append(line, row.Total))
	}
	writer.Flush()
	return buffer.Bytes()
}

// LiveScoreboard 依据当前提交实时计算成绩单（不依赖“生成成绩单”快照）。
func (s Service) LiveScoreboard(ctx context.Context, contestID int64) (Scoreboard, error) {
	return s.buildScoreboard(ctx, contestID)
}

// EvalCell 是评估表格中（某考生，某题）的单元格。
type EvalCell struct {
	HasSubmission bool   `json:"has_submission"`
	SubmissionID  int64  `json:"submission_id,omitempty"`
	Status        string `json:"status,omitempty"`
	Score         string `json:"score,omitempty"`
}

type EvalRow struct {
	HasSubmission bool       `json:"has_submission"`
	Rank          int        `json:"rank"`
	HasScore      bool       `json:"has_score"`
	StudentID     string     `json:"student_id"`
	Name          string     `json:"name"`
	Cells         []EvalCell `json:"cells"`
	Total         string     `json:"total"`
}

// Evaluation 是比赛评测页所需的实时二维成绩表（考生 × 题目）。
type Evaluation struct {
	ContestID int64            `json:"contest_id"`
	Problems  []ContestProblem `json:"problems"`
	Rows      []EvalRow        `json:"rows"`
	Pending   int              `json:"pending"` // 仍在等待/评测中的提交数
}

// Evaluation 实时构建评测表格：每格取该考生该题的最后一次提交（含状态与分数）。
func (s Service) Evaluation(ctx context.Context, contestID int64) (Evaluation, error) {
	result := Evaluation{ContestID: contestID}

	rows, err := s.Store.DB.QueryContext(ctx,
		`SELECT cp.problem_id,cp.label,COALESCE(p.title,''),cp.score_cents FROM contest_problems cp
		 JOIN problems p ON p.id=cp.problem_id WHERE cp.contest_id=? ORDER BY cp.order_index`, contestID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item ContestProblem
		var scoreCents int64
		if err := rows.Scan(&item.ProblemID, &item.Label, &item.Title, &scoreCents); err != nil {
			rows.Close()
			return result, err
		}
		item.Score = store.FormatScore(scoreCents)
		result.Problems = append(result.Problems, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return result, err
	}

	// pending 只统计真正在评测队列中（queued/running）的提交；导入后尚未入队的提交不计入。
	if err := s.Store.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM submissions s
		 JOIN contest_problems cp ON cp.problem_id=s.problem_id AND cp.contest_id=?
		 JOIN judge_queue q ON q.submission_id=s.id
		 WHERE s.contestant_id IS NOT NULL AND q.state IN ('queued','running')`, contestID).Scan(&result.Pending); err != nil {
		return result, err
	}

	crows, err := s.Store.DB.QueryContext(ctx, `SELECT id,student_id,name FROM contestants WHERE contest_id=? ORDER BY student_id`, contestID)
	if err != nil {
		return result, err
	}
	defer crows.Close()
	for crows.Next() {
		var contestantID int64
		row := EvalRow{}
		if err := crows.Scan(&contestantID, &row.StudentID, &row.Name); err != nil {
			return result, err
		}
		var total int64
		hasScore := false
		for _, problem := range result.Problems {
			var cell EvalCell
			var submissionID sql.NullInt64
			var status string
			var score sql.NullInt64
			err := s.Store.DB.QueryRowContext(ctx,
				`SELECT id,status,score FROM submissions WHERE contestant_id=? AND problem_id=? ORDER BY id DESC LIMIT 1`,
				contestantID, problem.ProblemID).Scan(&submissionID, &status, &score)
			if errors.Is(err, sql.ErrNoRows) {
				row.Cells = append(row.Cells, EvalCell{})
				continue
			}
			if err != nil {
				return result, err
			}
			row.HasSubmission = true
			cell.HasSubmission = true
			cell.SubmissionID = submissionID.Int64
			cell.Status = status
			if score.Valid {
				cell.Score = store.FormatScore(score.Int64)
				total += score.Int64
				hasScore = true
			}
			row.Cells = append(row.Cells, cell)
		}
		row.HasScore = hasScore
		if hasScore {
			row.Total = store.FormatScore(total)
		} else {
			row.Total = "-"
		}
		result.Rows = append(result.Rows, row)
	}
	if err := crows.Err(); err != nil {
		return result, err
	}

	// 有成绩者按总分排名；未出成绩时，有提交者优先，无提交者置后，各组内按学号排序。
	scored := make([]EvalRow, 0, len(result.Rows))
	unscored := make([]EvalRow, 0, len(result.Rows))
	for _, row := range result.Rows {
		if row.HasScore {
			scored = append(scored, row)
		} else {
			unscored = append(unscored, row)
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		ai, _ := store.ParseScore(scored[i].Total)
		aj, _ := store.ParseScore(scored[j].Total)
		if ai != aj {
			return ai > aj
		}
		return scored[i].StudentID < scored[j].StudentID
	})
	sort.SliceStable(unscored, func(i, j int) bool {
		if unscored[i].HasSubmission != unscored[j].HasSubmission {
			return unscored[i].HasSubmission
		}
		return unscored[i].StudentID < unscored[j].StudentID
	})
	last := ""
	rank := 0
	for i := range scored {
		if scored[i].Total != last {
			rank = i + 1
			last = scored[i].Total
		}
		scored[i].Rank = rank
	}
	result.Rows = append(scored, unscored...)
	return result, nil
}

func (s Service) Audit(ctx context.Context, actorID int64, action, objectType, objectID string, detail map[string]any) error {
	encoded, _ := json.Marshal(detail)
	_, err := s.Store.DB.ExecContext(ctx,
		`INSERT INTO audit_logs(account_id,action,object_type,object_id,detail_json,created_at) VALUES(?,?,?,?,?,?)`,
		actorID, action, objectType, objectID, string(encoded), store.Now())
	return err
}

func auditTx(ctx context.Context, tx *sql.Tx, actorID int64, action, objectType, objectID string, detail map[string]any) error {
	encoded, _ := json.Marshal(detail)
	_, err := tx.ExecContext(ctx,
		`INSERT INTO audit_logs(account_id,action,object_type,object_id,detail_json,created_at) VALUES(?,?,?,?,?,?)`,
		actorID, action, objectType, objectID, string(encoded), store.Now())
	return err
}

func extension(language string) string {
	switch language {
	case "cpp":
		return "cpp"
	case "java":
		return "java"
	case "python3":
		return "py"
	}
	return ""
}

func languageForExtension(ext string) string {
	switch ext {
	case "cpp":
		return "cpp"
	case "java":
		return "java"
	case "py":
		return "python3"
	}
	return ""
}

// PackageFileName 生成便于识别、可在 Windows 保存的文件名。
func PackageFileName(kind, name string, id int64) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '-'
		}
		return r
	}, name)
	runes := []rune(strings.Trim(name, " ."))
	if len(runes) > 64 {
		runes = runes[:64]
	}
	return fmt.Sprintf("%s-%s-%d.zip", kind, string(runes), id)
}
