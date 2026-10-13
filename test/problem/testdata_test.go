package problem_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"justcpc-online-judge/src/problem"
	"justcpc-online-judge/src/store"
)

func TestTestdataMustBeSequentialPairs(t *testing.T) {
	root := t.TempDir()
	db, err := store.Open(filepath.Join(root, "judge.sqlite"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(context.Background(), `INSERT INTO accounts(id,username,password_hash,role,created_at,updated_at) VALUES(1,'owner','x','user',?,?)`, store.Now(), store.Now()); err != nil {
		t.Fatal(err)
	}
	service := problem.Service{Store: db}
	item, err := service.Create(context.Background(), 1, problem.CreateInput{Title: "test", Score: "100"})
	if err != nil {
		t.Fatal(err)
	}
	valid := archive(t, map[string]string{"1.in": "1\n", "1.out": "1\n", "2.in": "2\n", "2.out": "2\n"})
	if err := service.UploadTestdata(context.Background(), item.ID, 1, valid); err != nil {
		t.Fatalf("valid archive rejected: %v", err)
	}
	invalid := archive(t, map[string]string{"1.in": "1\n", "1.out": "1\n", "3.in": "3\n", "3.out": "3\n"})
	if err := service.UploadTestdata(context.Background(), item.ID, 1, invalid); err == nil {
		t.Fatal("non-sequential archive accepted")
	}
}

func TestOnlyOwnerCanReadEditorData(t *testing.T) {
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
	for id, username := range map[int]string{1: "owner", 2: "reader"} {
		if _, err := db.DB.ExecContext(ctx, `INSERT INTO accounts(id,username,password_hash,role,created_at,updated_at) VALUES(?,?, 'x','user',?,?)`, id, username, store.Now(), store.Now()); err != nil {
			t.Fatal(err)
		}
	}
	service := problem.Service{Store: db}
	item, err := service.Create(ctx, 1, problem.CreateInput{Title: "private configuration", StatementMarkdown: "# public statement", Score: "100"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UploadTestdata(ctx, item.ID, 1, archive(t, map[string]string{"1.in": "1\n", "1.out": "1\n"})); err != nil {
		t.Fatal(err)
	}
	list, err := service.ListPublic(ctx, 2)
	if err != nil || len(list) != 1 || list[0].Score != "100" || list[0].CanEdit || !list[0].AcceptingSubmissions {
		t.Fatalf("unexpected public list: %#v, %v", list, err)
	}
	if _, err := service.Editor(ctx, item.ID, 2); !errors.Is(err, problem.ErrForbidden) {
		t.Fatalf("non-owner editor access: %v", err)
	}
	editor, err := service.Editor(ctx, item.ID, 1)
	if err != nil || editor.TestCaseCount != 1 || !editor.TestdataReady {
		t.Fatalf("owner editor data: %#v, %v", editor, err)
	}
}

func archive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, value := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(value)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
