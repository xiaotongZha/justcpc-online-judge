package contest_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"justcpc-online-judge/src/contest"
	"justcpc-online-judge/src/store"
)

func TestContestCreateExportAndImportSubmissions(t *testing.T) {
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
	// 两道既有题目（含测试数据）
	for _, id := range []int64{1, 2} {
		if _, err := db.DB.ExecContext(ctx,
			`INSERT INTO problems(id,title,statement_markdown,score,resource_limits_json,testdata_path,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
			id, fmt.Sprintf("题目%d", id), "", 10000, `{}`, fmt.Sprintf("tests/%d", id), store.Now(), store.Now()); err != nil {
			t.Fatal(err)
		}
		if err := db.WriteFile(fmt.Sprintf("tests/%d/1.in", id), []byte("1\n")); err != nil {
			t.Fatal(err)
		}
		if err := db.WriteFile(fmt.Sprintf("tests/%d/1.out", id), []byte("1\n")); err != nil {
			t.Fatal(err)
		}
	}
	service := contest.Service{Store: db}
	item, err := service.Create(ctx, 1, contest.CreateInput{
		Name:     "测试赛",
		Problems: []contest.CreateProblem{{ProblemID: 1, Score: "100"}, {ProblemID: 2, Score: "50"}},
	}, []byte("%PDF-1.4\n"), "roster.csv", []byte("student_id,name,password\n20260001,张三,pw1\n20260002,李四,pw2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if item.ProblemCount != 2 || item.ContestantCount != 2 {
		t.Fatalf("unexpected contest: %#v", item)
	}

	// 导出考试包
	_, data, err := service.ExportExamPackage(ctx, 1, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	entries := zipEntries(t, data)
	for _, name := range []string{"contest.json", "statement.pdf", "contestants.csv"} {
		if _, ok := entries[name]; !ok {
			t.Fatalf("exam package missing %s", name)
		}
	}

	// 构造提交包并导入
	submission := buildSubmissionPackage(t, item.ID)
	result, err := service.ImportSubmissions(ctx, 1, item.ID, submission)
	if err != nil {
		t.Fatal(err)
	}
	if result.Contestants != 1 || result.Submissions != 1 {
		t.Fatalf("unexpected import result: %#v", result)
	}

	// 模拟评测完成
	if _, err := db.DB.ExecContext(ctx, `UPDATE submissions SET status='AC',score=10000`); err != nil {
		t.Fatal(err)
	}
	board, err := service.LiveScoreboard(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Rows) != 3 || len(board.Problems) != 2 {
		t.Fatalf("unexpected board: %#v", board)
	}
	if board.Rows[0].StudentID != "20260003" || board.Rows[0].Total != "100" {
		t.Fatalf("unexpected first row: %#v", board.Rows[0])
	}
	if board.Problems[0].Label != "A" || board.Problems[1].Label != "B" || board.Problems[1].Score != "50" {
		t.Fatalf("unexpected problems: %#v", board.Problems)
	}

	// 比赛列表应带派生计数
	list, err := service.List(ctx)
	if err != nil || len(list) != 1 || list[0].SubmissionCount != 1 {
		t.Fatalf("unexpected list: %#v, %v", list, err)
	}
}

func buildSubmissionPackage(t *testing.T, contestID int64) []byte {
	t.Helper()
	manifest := map[string]any{
		"contest_id": contestID,
		"name":       "测试赛",
		"problems": []map[string]any{
			{"problem_id": 1, "label": "A"},
			{"problem_id": 2, "label": "B"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	write := func(name string, value []byte) {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(value); err != nil {
			t.Fatal(err)
		}
	}
	write("contest.json", manifestData)
	write("contestants.csv", []byte("\ufeffstudent_id,name\n20260001,张三\n20260002,李四\n20260003,王五\n"))
	write("submissions/20260003/A/source.cpp", []byte("int main(){}"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func zipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	result := map[string][]byte{}
	for _, entry := range reader.File {
		file, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content := make([]byte, entry.UncompressedSize64)
		if _, err := file.Read(content); err != nil && err.Error() != "EOF" {
			file.Close()
			t.Fatal(err)
		}
		file.Close()
		result[entry.Name] = content
	}
	return result
}
