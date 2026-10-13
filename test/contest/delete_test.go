package contest_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"justcpc-online-judge/src/contest"
	"justcpc-online-judge/src/store"
)

func TestContestDeleteCascadeAndCleanup(t *testing.T) {
	root := t.TempDir()
	db, err := store.Open(filepath.Join(root, "judge.sqlite"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO accounts(id,username,password_hash,role,created_at,updated_at) VALUES(1,'admin','x','admin',?,?)`, store.Now(), store.Now()); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{1, 2} {
		if _, err := db.DB.ExecContext(ctx,
			`INSERT INTO problems(id,title,statement_markdown,score,resource_limits_json,testdata_path,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
			id, fmt.Sprintf("题目%d", id), "", 10000, `{}`, fmt.Sprintf("tests/%d", id), store.Now(), store.Now()); err != nil {
			t.Fatal(err)
		}
	}
	service := contest.Service{Store: db}
	item, err := service.Create(ctx, 1, contest.CreateInput{
		Name:     "待删比赛",
		Problems: []contest.CreateProblem{{ProblemID: 1, Score: "100"}, {ProblemID: 2, Score: "50"}},
	}, []byte("%PDF-1.4\n"), "roster.csv", []byte("student_id,name,password\n20260001,张三,pw1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ImportSubmissions(ctx, 1, item.ID, buildSubmissionPackage(t, item.ID)); err != nil {
		t.Fatal(err)
	}
	var submissionID int64
	if err := db.DB.QueryRowContext(ctx, `SELECT id FROM submissions ORDER BY id LIMIT 1`).Scan(&submissionID); err != nil {
		t.Fatalf("expected an imported submission: %v", err)
	}

	if err := service.Delete(ctx, 1, item.ID); err != nil {
		t.Fatalf("delete contest: %v", err)
	}

	if _, err := service.Get(ctx, item.ID); err == nil {
		t.Fatal("contest still present after delete")
	}
	for name, query := range map[string]string{
		"contests":         `SELECT COUNT(*) FROM contests`,
		"contestants":      `SELECT COUNT(*) FROM contestants`,
		"contest_problems": `SELECT COUNT(*) FROM contest_problems`,
		"submissions":      `SELECT COUNT(*) FROM submissions`,
	} {
		var count int
		if err := db.DB.QueryRowContext(ctx, query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s not cleaned: %d remain", name, count)
		}
	}
	var problems int
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM problems`).Scan(&problems); err != nil {
		t.Fatal(err)
	}
	if problems != 2 {
		t.Fatalf("shared problems should remain, got %d", problems)
	}
	for _, dir := range []string{
		filepath.Join("contest-statements", fmt.Sprint(item.ID)),
		filepath.Join("contest-rosters", fmt.Sprint(item.ID)),
		filepath.Join("submissions", fmt.Sprint(submissionID)),
	} {
		if _, err := os.Stat(db.Absolute(dir)); !os.IsNotExist(err) {
			t.Fatalf("directory not removed: %s (%v)", dir, err)
		}
	}
}
