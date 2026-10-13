package problem_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"justcpc-online-judge/src/problem"
	"justcpc-online-judge/src/store"
)

func TestDeleteProblemPermissionsAndCleanup(t *testing.T) {
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
	for id, seed := range map[int]struct{ username, role string }{
		1: {"owner", "user"},
		2: {"other", "user"},
		3: {"root", "admin"},
	} {
		if _, err := db.DB.ExecContext(ctx, `INSERT INTO accounts(id,username,password_hash,role,created_at,updated_at) VALUES(?,?,'x',?,?,?)`, id, seed.username, seed.role, store.Now(), store.Now()); err != nil {
			t.Fatal(err)
		}
	}
	service := problem.Service{Store: db}

	own, err := service.Create(ctx, 1, problem.CreateInput{Title: "own", Score: "100"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, own.ID, 1, false); err != nil {
		t.Fatalf("owner should delete own problem: %v", err)
	}
	if _, err := service.Get(ctx, own.ID); err == nil {
		t.Fatal("own problem still present after delete")
	}

	item, err := service.Create(ctx, 1, problem.CreateInput{Title: "deletable", Score: "100"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UploadTestdata(ctx, item.ID, 1, archive(t, map[string]string{"1.in": "1\n", "1.out": "1\n"})); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join("submissions", "1", "main.cpp")
	result, err := db.DB.ExecContext(ctx, `INSERT INTO submissions(problem_id,account_id,language,source_path,status,submitted_at) VALUES(?,?,'cpp',?,'waiting',?)`, item.ID, 1, source, store.Now())
	if err != nil {
		t.Fatal(err)
	}
	submissionID, _ := result.LastInsertId()
	if err := db.WriteFile(source, []byte("int main(){}")); err != nil {
		t.Fatal(err)
	}

	if err := service.Delete(ctx, item.ID, 2, false); !errors.Is(err, problem.ErrForbidden) {
		t.Fatalf("non-owner should be forbidden, got %v", err)
	}
	if _, err := service.Get(ctx, item.ID); err != nil {
		t.Fatalf("problem must survive forbidden delete: %v", err)
	}

	if err := service.Delete(ctx, item.ID, 3, true); err != nil {
		t.Fatalf("admin should delete any problem: %v", err)
	}
	if _, err := service.Get(ctx, item.ID); err == nil {
		t.Fatal("problem still present after admin delete")
	}
	var count int
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM submissions WHERE problem_id=?`, item.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("submissions not cascaded: %d remain", count)
	}
	if _, err := os.Stat(db.Absolute(filepath.Join("tests", fmt.Sprint(item.ID)))); !os.IsNotExist(err) {
		t.Fatalf("testdata directory not removed: %v", err)
	}
	if _, err := os.Stat(db.Absolute(filepath.Join("submissions", fmt.Sprint(submissionID)))); !os.IsNotExist(err) {
		t.Fatalf("submission files not removed: %v", err)
	}
}
