package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const TimeLayout = "2006-01-02 15:04:05"

var beijing = time.FixedZone("Asia/Shanghai", 8*60*60)

type Store struct {
	DB       *sql.DB
	DataRoot string
}

func Open(databasePath, dataRoot string) (*Store, error) {
	absoluteDataRoot, err := filepath.Abs(dataRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	dataRoot = absoluteDataRoot
	if err := os.MkdirAll(filepath.Dir(databasePath), 0700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	if err := os.MkdirAll(dataRoot, 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open SQLite: %w", err)
	}
	store := &Store{DB: db, DataRoot: dataRoot}
	if err := store.Migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, password_text TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL CHECK(role IN ('admin','user')), created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS contests (
			id INTEGER PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			statement_pdf_path TEXT, roster_path TEXT, created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS contestants (
			id INTEGER PRIMARY KEY, contest_id INTEGER NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
			student_id TEXT NOT NULL, name TEXT NOT NULL, created_at TEXT NOT NULL,
			UNIQUE(contest_id, student_id)
		)`,
		`CREATE TABLE IF NOT EXISTS problems (
			id INTEGER PRIMARY KEY, title TEXT, statement_markdown TEXT,
			owner_account_id INTEGER REFERENCES accounts(id),
			score INTEGER NOT NULL, resource_limits_json TEXT, compare_json TEXT NOT NULL DEFAULT '{}', testdata_path TEXT,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS submissions (
			id INTEGER PRIMARY KEY, problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
			account_id INTEGER REFERENCES accounts(id), contestant_id INTEGER REFERENCES contestants(id),
			language TEXT NOT NULL CHECK(language IN ('cpp','java','python3')), source_path TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('waiting','AC','WA','CE')), score INTEGER,
			judge_result_json TEXT, submitted_at TEXT NOT NULL, judged_at TEXT,
			CHECK((account_id IS NOT NULL AND contestant_id IS NULL) OR (account_id IS NULL AND contestant_id IS NOT NULL))
		)`,
		`CREATE TABLE IF NOT EXISTS judge_queue (
			id INTEGER PRIMARY KEY, submission_id INTEGER NOT NULL UNIQUE REFERENCES submissions(id) ON DELETE CASCADE,
			state TEXT NOT NULL CHECK(state IN ('queued','running','failed')), attempt INTEGER NOT NULL DEFAULT 0,
			lease_until TEXT, queued_at TEXT NOT NULL, started_at TEXT, error TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS contest_problems (
			contest_id INTEGER NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
			problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
			label TEXT NOT NULL, order_index INTEGER NOT NULL, score_cents INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (contest_id, problem_id)
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY, account_id INTEGER REFERENCES accounts(id), action TEXT NOT NULL,
			object_type TEXT NOT NULL, object_id INTEGER, detail_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS submissions_problem_idx ON submissions(problem_id, submitted_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS submissions_contestant_idx ON submissions(contestant_id, submitted_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS judge_queue_state_idx ON judge_queue(state, lease_until, queued_at)`,
	}
	for _, statement := range statements {
		if _, err := s.DB.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("run migration: %w", err)
		}
	}
	// Existing development databases from before floating-point comparison support
	// receive the column without requiring a manual data migration.
	if _, err := s.DB.ExecContext(ctx, `ALTER TABLE problems ADD COLUMN compare_json TEXT NOT NULL DEFAULT '{}'`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("add problem comparison options: %w", err)
	}
	if _, err := s.DB.ExecContext(ctx, `ALTER TABLE accounts ADD COLUMN password_text TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("add account password text: %w", err)
	}
	for _, column := range []string{
		`ALTER TABLE contests ADD COLUMN roster_path TEXT`,
	} {
		if _, err := s.DB.ExecContext(ctx, column); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("alter contests: %w", err)
		}
	}
	return s.rebuildSchema(ctx)
}

// rebuildSchema 删除历史上已废弃的表与列：contest_scoreboards 表，
// problems 的 contest_id/statement_images_json/statement_pdf_path，contests 的
// problem_ids_json/exam_start_at/exam_end_at。SQLite 无法直接删列，故重建表。
func (s *Store) rebuildSchema(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `DROP TABLE IF EXISTS contest_scoreboards`); err != nil {
		return fmt.Errorf("drop contest_scoreboards: %w", err)
	}
	if err := s.rebuildTable(ctx, "problems",
		[]string{"contest_id", "statement_images_json", "statement_pdf_path"},
		`CREATE TABLE problems_new (
			id INTEGER PRIMARY KEY, title TEXT, statement_markdown TEXT,
			owner_account_id INTEGER REFERENCES accounts(id),
			score INTEGER NOT NULL, resource_limits_json TEXT, compare_json TEXT NOT NULL DEFAULT '{}', testdata_path TEXT,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`INSERT INTO problems_new(id,title,statement_markdown,owner_account_id,score,resource_limits_json,compare_json,testdata_path,created_at,updated_at)
		 SELECT id,title,statement_markdown,owner_account_id,score,resource_limits_json,compare_json,testdata_path,created_at,updated_at FROM problems`,
	); err != nil {
		return err
	}
	return s.rebuildTable(ctx, "contests",
		[]string{"problem_ids_json", "exam_start_at", "exam_end_at"},
		`CREATE TABLE contests_new (
			id INTEGER PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			statement_pdf_path TEXT, roster_path TEXT, created_at TEXT NOT NULL)`,
		`INSERT INTO contests_new(id,name,description,statement_pdf_path,roster_path,created_at)
		 SELECT id,name,description,statement_pdf_path,roster_path,created_at FROM contests`)
}

// rebuildTable 当表仍带有任一 dead 列时，按 新表->复制->丢旧表->改名 重建它。
func (s *Store) rebuildTable(ctx context.Context, table string, dead []string, create, copyData string) error {
	need := false
	for _, column := range dead {
		exists, err := s.columnExists(ctx, table, column)
		if err != nil {
			return err
		}
		if exists {
			need = true
			break
		}
	}
	if !need {
		return nil
	}
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`)
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, statement := range []string{create, copyData, `DROP TABLE ` + table, `ALTER TABLE ` + table + `_new RENAME TO ` + table} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("rebuild %s table: %w", table, err)
		}
	}
	return tx.Commit()
}

func (s *Store) columnExists(ctx context.Context, table, column string) (bool, error) {
	rows, err := s.DB.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func Now() string { return time.Now().In(beijing).Format(TimeLayout) }

func NowAfter(duration time.Duration) string {
	return time.Now().In(beijing).Add(duration).Format(TimeLayout)
}

func WithTx[T any](ctx context.Context, db *sql.DB, fn func(*sql.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return zero, err
	}
	value, err := fn(tx)
	if err != nil {
		_ = tx.Rollback()
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return value, nil
}

func FormatScore(cents int64) string {
	whole := cents / 100
	fraction := cents % 100
	if fraction < 0 {
		fraction = -fraction
	}
	if fraction == 0 {
		return fmt.Sprintf("%d", whole)
	}
	if fraction%10 == 0 {
		return fmt.Sprintf("%d.%d", whole, fraction/10)
	}
	return fmt.Sprintf("%d.%02d", whole, fraction)
}

func ParseScore(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || value == "" {
		return 0, fmt.Errorf("invalid score %q", value)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, fmt.Errorf("invalid score %q", value)
	}
	if len(parts) == 1 {
		return whole * 100, nil
	}
	if len(parts[1]) == 0 || len(parts[1]) > 2 {
		return 0, fmt.Errorf("score must have at most two decimal places")
	}
	fraction, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid score %q", value)
	}
	if len(parts[1]) == 1 {
		fraction *= 10
	}
	return whole*100 + fraction, nil
}
