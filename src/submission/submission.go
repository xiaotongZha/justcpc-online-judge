package submission

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"justcpc-online-judge/src/problem"
	"justcpc-online-judge/src/store"
)

var ErrForbidden = errors.New("permission denied")

type Submission struct {
	ID           int64           `json:"id"`
	ProblemID    int64           `json:"problem_id"`
	ProblemTitle string          `json:"problem_title"`
	IsContestant bool            `json:"is_contestant"`
	Submitter    string          `json:"submitter"`
	Language     string          `json:"language"`
	Status       string          `json:"status"`
	Score        int64           `json:"-"`
	ScoreText    string          `json:"score"`
	SubmittedAt  string          `json:"submitted_at"`
	JudgedAt     *string         `json:"judged_at,omitempty"`
	Source       string          `json:"source,omitempty"`
	JudgeResult  json.RawMessage `json:"judge_result_json,omitempty"`
}

type CreateInput struct {
	Language string `json:"language"`
	Source   string `json:"source"`
}
type Service struct {
	Store    *store.Store
	Problems problem.Service
}

func (s Service) Create(ctx context.Context, problemID, accountID int64, input CreateInput) (Submission, error) {
	if extension(input.Language) == "" {
		return Submission{}, errors.New("language must be cpp, java, or python3")
	}
	if input.Source == "" {
		return Submission{}, errors.New("source is required")
	}
	p, err := s.Problems.Get(ctx, problemID)
	if err != nil {
		return Submission{}, err
	}
	if p.TestdataPath == "" {
		return Submission{}, errors.New("problem has no test data")
	}
	now := store.Now()
	var id int64
	_, err = store.WithTx(ctx, s.Store.DB, func(tx *sql.Tx) (struct{}, error) {
		result, err := tx.ExecContext(ctx, `INSERT INTO submissions(problem_id,account_id,language,source_path,status,submitted_at) VALUES(?,?,?,?,?,?)`, problemID, accountID, input.Language, "", "waiting", now)
		if err != nil {
			return struct{}{}, err
		}
		id, _ = result.LastInsertId()
		relative := filepath.Join("submissions", fmt.Sprint(id), "main."+extension(input.Language))
		if _, err = tx.ExecContext(ctx, `UPDATE submissions SET source_path=? WHERE id=?`, relative, id); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err != nil {
		return Submission{}, err
	}
	relative := filepath.Join("submissions", fmt.Sprint(id), "main."+extension(input.Language))
	if err := s.Store.WriteFile(relative, []byte(input.Source)); err != nil {
		_, _ = s.Store.DB.ExecContext(ctx, `DELETE FROM submissions WHERE id=?`, id)
		return Submission{}, err
	}
	if _, err := s.Store.DB.ExecContext(ctx, `INSERT INTO judge_queue(submission_id,state,queued_at) VALUES(?,?,?)`, id, "queued", now); err != nil {
		_ = os.Remove(s.Store.Absolute(relative))
		_, _ = s.Store.DB.ExecContext(ctx, `DELETE FROM submissions WHERE id=?`, id)
		return Submission{}, err
	}
	return s.Detail(ctx, id, accountID, false)
}

// SubmissionQuery 是提交列表的筛选与分页条件。
type SubmissionQuery struct {
	Status    string
	Language  string
	ProblemID int64
	Keyword   string
	Page      int
	PageSize  int
}

type SubmissionPage struct {
	Items    []Submission `json:"items"`
	Total    int          `json:"total"`
	Waiting  int          `json:"waiting"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

// Page 按条件分页返回提交记录，并附带总数与全局等待判题数。
func (s Service) Page(ctx context.Context, query SubmissionQuery) (SubmissionPage, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	size := query.PageSize
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}

	where := []string{}
	args := []any{}
	if query.Status != "" && query.Status != "all" {
		where = append(where, "s.status=?")
		args = append(args, query.Status)
	}
	if query.Language != "" && query.Language != "all" {
		where = append(where, "s.language=?")
		args = append(args, query.Language)
	}
	if query.ProblemID > 0 {
		where = append(where, "s.problem_id=?")
		args = append(args, query.ProblemID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		where = append(where, "(p.title LIKE ? OR COALESCE(a.username,ct.student_id) LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	base := ` FROM submissions s JOIN problems p ON p.id=s.problem_id
		LEFT JOIN accounts a ON a.id=s.account_id LEFT JOIN contestants ct ON ct.id=s.contestant_id`

	result := SubmissionPage{Page: page, PageSize: size, Items: []Submission{}}
	if err := s.Store.DB.QueryRowContext(ctx, "SELECT COUNT(*)"+base+clause, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM submissions WHERE status='waiting'`).Scan(&result.Waiting); err != nil {
		return result, err
	}

	selectArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := s.Store.DB.QueryContext(ctx,
		`SELECT s.id,s.problem_id,COALESCE(p.title,''),COALESCE(a.username,ct.student_id),
			CASE WHEN s.contestant_id IS NOT NULL THEN 1 ELSE 0 END,s.language,s.status,
			COALESCE(s.score,0),s.submitted_at,s.judged_at`+base+clause+
			` ORDER BY s.id DESC LIMIT ? OFFSET ?`, selectArgs...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Submission
		var judged sql.NullString
		var contestant int
		if err := rows.Scan(&item.ID, &item.ProblemID, &item.ProblemTitle, &item.Submitter, &contestant, &item.Language, &item.Status, &item.Score, &item.SubmittedAt, &judged); err != nil {
			return result, err
		}
		if judged.Valid {
			item.JudgedAt = &judged.String
		}
		item.IsContestant = contestant != 0
		item.ScoreText = store.FormatScore(item.Score)
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}

func (s Service) Detail(ctx context.Context, id, accountID int64, isAdmin bool) (Submission, error) {
	var item Submission
	var owner, submitter sql.NullInt64
	var sourcePath string
	var result sql.NullString
	var judged sql.NullString
	err := s.Store.DB.QueryRowContext(ctx, `SELECT s.id,s.problem_id,COALESCE(p.title,''),COALESCE(a.username,ct.student_id),s.language,s.status,COALESCE(s.score,0),s.submitted_at,s.judged_at,s.source_path,s.judge_result_json,p.owner_account_id,s.account_id FROM submissions s JOIN problems p ON p.id=s.problem_id LEFT JOIN accounts a ON a.id=s.account_id LEFT JOIN contestants ct ON ct.id=s.contestant_id WHERE s.id=?`, id).Scan(&item.ID, &item.ProblemID, &item.ProblemTitle, &item.Submitter, &item.Language, &item.Status, &item.Score, &item.SubmittedAt, &judged, &sourcePath, &result, &owner, &submitter)
	if err != nil {
		return item, err
	}
	if !isAdmin && (!submitter.Valid || submitter.Int64 != accountID) && (!owner.Valid || owner.Int64 != accountID) {
		return Submission{}, ErrForbidden
	}
	data, err := os.ReadFile(s.Store.Absolute(sourcePath))
	if err != nil {
		return Submission{}, err
	}
	item.Source = string(data)
	item.ScoreText = store.FormatScore(item.Score)
	if judged.Valid {
		item.JudgedAt = &judged.String
	}
	if result.Valid {
		item.JudgeResult = json.RawMessage(result.String)
	}
	return item, nil
}

func (s Service) LoadExecution(ctx context.Context, submissionID int64) (Submission, problem.Problem, string, error) {
	var p problem.Problem
	var language, sourcePath string
	var score int64
	var limits string
	var compare string
	var owner sql.NullInt64
	err := s.Store.DB.QueryRowContext(ctx, `SELECT s.language,s.source_path,p.id,p.title,p.statement_markdown,p.score,p.resource_limits_json,COALESCE(p.compare_json,'{}'),COALESCE(p.testdata_path,''),p.owner_account_id,p.created_at,p.updated_at FROM submissions s JOIN problems p ON p.id=s.problem_id WHERE s.id=?`, submissionID).Scan(&language, &sourcePath, &p.ID, &p.Title, &p.StatementMarkdown, &score, &limits, &compare, &p.TestdataPath, &owner, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Submission{}, p, "", err
	}
	if owner.Valid {
		p.OwnerAccountID = &owner.Int64
	}
	p.Score = score
	p.ScoreText = store.FormatScore(score)
	_ = json.Unmarshal([]byte(limits), &p.ResourceLimits)
	_ = json.Unmarshal([]byte(compare), &p.Compare)
	p.ResourceLimits = problem.DefaultLimits()
	if limits != "" {
		var override problem.Limits
		_ = json.Unmarshal([]byte(limits), &override)
		for key, value := range override {
			p.ResourceLimits[key] = value
		}
	}
	source, err := os.ReadFile(s.Store.Absolute(sourcePath))
	if err != nil {
		return Submission{}, p, "", err
	}
	return Submission{ID: submissionID, ProblemID: p.ID, Language: language}, p, string(source), nil
}

func (s Service) SaveResult(ctx context.Context, submissionID int64, status string, score int64, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = s.Store.DB.ExecContext(ctx, `UPDATE submissions SET status=?,score=?,judge_result_json=?,judged_at=? WHERE id=?`, status, score, string(encoded), store.Now(), submissionID)
	return err
}
func (s Service) MarkQueueFailed(ctx context.Context, queueID int64, message string) error {
	_, err := s.Store.DB.ExecContext(ctx, `UPDATE judge_queue SET state='failed',error=?,lease_until=NULL WHERE id=?`, message, queueID)
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
