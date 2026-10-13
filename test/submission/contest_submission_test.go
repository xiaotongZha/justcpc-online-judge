package submission_test

import (
	"context"
	"path/filepath"
	"testing"

	"justcpc-online-judge/src/problem"
	"justcpc-online-judge/src/store"
	"justcpc-online-judge/src/submission"
)

func TestOrdinaryAccountCanSubmitProblem(t *testing.T) {
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
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO accounts(id,username,password_hash,role,created_at,updated_at) VALUES(1,'player','x','user',?,?)`, store.Now(), store.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO problems(id,title,score,resource_limits_json,testdata_path,created_at,updated_at) VALUES(1,'A',10000,'{}','tests/1',?,?)`, store.Now(), store.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.WriteFile("tests/1/1.in", []byte("1\n")); err != nil {
		t.Fatal(err)
	}
	if err := db.WriteFile("tests/1/1.out", []byte("1\n")); err != nil {
		t.Fatal(err)
	}
	service := submission.Service{Store: db, Problems: problem.Service{Store: db}}
	item, err := service.Create(ctx, 1, 1, submission.CreateInput{Language: "cpp", Source: "int main(){}"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != "waiting" {
		t.Fatalf("unexpected status %q", item.Status)
	}
	var queued int
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM judge_queue WHERE submission_id=? AND state='queued'`, item.ID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("contest submission queue: %d, %v", queued, err)
	}
}
