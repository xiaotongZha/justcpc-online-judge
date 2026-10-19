package contest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ImportMapping 仅用于缺少平台 ID 的历史提交包，必须明确指定目标比赛及题号对应关系。
type ImportMapping struct {
	ContestID int64            `json:"contest_id"`
	Problems  map[string]int64 `json:"problems"`
}

type ImportProblemPreview struct {
	Label            string `json:"label"`
	Title            string `json:"title"`
	PackageProblemID int64  `json:"package_problem_id"`
	ProblemID        int64  `json:"problem_id"`
}

type SubmissionPackagePreview struct {
	Name             string                 `json:"name"`
	TargetName       string                 `json:"target_name"`
	ContestID        int64                  `json:"contest_id"`
	PackageContestID int64                  `json:"package_contest_id"`
	Contestants      int                    `json:"contestants"`
	Submissions      int                    `json:"submissions"`
	RequiresMapping  bool                   `json:"requires_mapping"`
	Problems         []ImportProblemPreview `json:"problems"`
}

type sourceInput struct {
	StudentID string
	Label     string
	Language  string
	Data      []byte
}

type parsedSubmissions struct {
	preview        SubmissionPackagePreview
	rows           [][]string
	sources        []sourceInput
	labelToProblem map[string]int64
}

func (s Service) PreviewSubmissions(ctx context.Context, contestID int64, archive []byte) (SubmissionPackagePreview, error) {
	data, err := s.readSubmissions(ctx, contestID, archive)
	return data.preview, err
}

func (s Service) readSubmissions(ctx context.Context, contestID int64, archive []byte) (parsedSubmissions, error) {
	var result parsedSubmissions
	detail, err := s.Detail(ctx, contestID)
	if err != nil {
		return result, err
	}
	files, err := readZipFiles(archive)
	if err != nil {
		return result, err
	}
	if _, ok := files["statement.pdf"]; ok {
		return result, errors.New("这是考试包，请到 Exam Server 导入；这里需要考试结束后导出的提交包")
	}
	var manifest submissionManifest
	if err := json.Unmarshal(files["contest.json"], &manifest); err != nil {
		return result, errors.New("提交包缺少有效的 contest.json")
	}
	if manifest.PackageType != "" && manifest.PackageType != "submissions" {
		return result, errors.New("包类型错误：这里仅接受 Exam Server 导出的提交包")
	}
	if manifest.ContestID < 0 || (manifest.ContestID != 0 && manifest.ContestID != contestID) {
		return result, fmt.Errorf("提交包属于比赛 #%d，与当前比赛 #%d 不一致", manifest.ContestID, contestID)
	}
	result.preview = SubmissionPackagePreview{Name: manifest.Name, TargetName: detail.Name, ContestID: contestID, PackageContestID: manifest.ContestID, RequiresMapping: manifest.ContestID == 0, Problems: []ImportProblemPreview{}}
	if len(manifest.Problems) == 0 {
		return result, errors.New("提交包没有题目映射")
	}
	localProblems := map[string]ContestProblem{}
	for _, p := range detail.Problems {
		localProblems[p.Label] = p
	}
	result.labelToProblem = map[string]int64{}
	for _, p := range manifest.Problems {
		local, ok := localProblems[p.Label]
		if _, duplicate := result.labelToProblem[p.Label]; duplicate {
			return result, fmt.Errorf("提交包题号 %s 重复", p.Label)
		}
		if !ok || p.ProblemID < 0 || (p.ProblemID != 0 && p.ProblemID != local.ProblemID) {
			return result, fmt.Errorf("题目 %s 与本地比赛不匹配", p.Label)
		}
		result.labelToProblem[p.Label] = local.ProblemID
		result.preview.RequiresMapping = result.preview.RequiresMapping || p.ProblemID == 0
		result.preview.Problems = append(result.preview.Problems, ImportProblemPreview{Label: p.Label, Title: local.Title, PackageProblemID: p.ProblemID, ProblemID: local.ProblemID})
	}
	csvBytes, ok := files["contestants.csv"]
	if !ok {
		return result, errors.New("提交包缺少 contestants.csv")
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(csvBytes), "\ufeff"))).ReadAll()
	if err != nil {
		return result, fmt.Errorf("contestants.csv：%w", err)
	}
	if len(rows) < 2 || len(rows[0]) < 2 || rows[0][0] != "student_id" || rows[0][1] != "name" {
		return result, errors.New("提交包名单表头应为 student_id,name，且至少有一名选手")
	}
	students := map[string]bool{}
	for i, row := range rows[1:] {
		if len(row) < 2 {
			return result, fmt.Errorf("名单第 %d 行缺少学号或姓名", i+2)
		}
		row[0], row[1] = strings.TrimSpace(row[0]), strings.TrimSpace(row[1])
		if row[0] == "" || row[1] == "" || students[row[0]] {
			return result, fmt.Errorf("名单第 %d 行学号、姓名为空或学号重复", i+2)
		}
		students[row[0]] = true
	}
	seen := map[string]bool{}
	for path, content := range files {
		if path == "contest.json" || path == "contestants.csv" {
			continue
		}
		parts := strings.Split(path, "/")
		if len(parts) != 4 || parts[0] != "submissions" || !strings.HasPrefix(parts[3], "source.") {
			return result, fmt.Errorf("提交包存在无法识别的文件 %s", path)
		}
		language := languageForExtension(strings.TrimPrefix(parts[3], "source."))
		if language == "" {
			return result, fmt.Errorf("不支持的源码文件 %s", path)
		}
		if !students[parts[1]] {
			return result, fmt.Errorf("提交中的学号 %s 不在名单中", parts[1])
		}
		if _, ok := result.labelToProblem[parts[2]]; !ok {
			return result, fmt.Errorf("提交题号 %s 缺少有效题目映射", parts[2])
		}
		key := parts[1] + "/" + parts[2]
		if seen[key] {
			return result, fmt.Errorf("%s 有多份代码，每人每题只能导入最后一份有效代码", key)
		}
		if len(content) == 0 {
			return result, fmt.Errorf("源码文件 %s 为空", path)
		}
		seen[key] = true
		result.sources = append(result.sources, sourceInput{StudentID: parts[1], Label: parts[2], Language: language, Data: content})
	}
	sort.Slice(result.sources, func(i, j int) bool {
		a, b := result.sources[i], result.sources[j]
		if a.StudentID == b.StudentID {
			return a.Label < b.Label
		}
		return a.StudentID < b.StudentID
	})
	result.rows = rows
	result.preview.Contestants = len(students)
	result.preview.Submissions = len(result.sources)
	return result, nil
}

func (p SubmissionPackagePreview) validateMapping(mapping ImportMapping) error {
	if !p.RequiresMapping {
		return nil
	}
	if mapping.ContestID != p.ContestID || len(mapping.Problems) != len(p.Problems) {
		return errors.New("历史提交包缺少平台 ID，请先预览并明确确认目标比赛和全部题目映射")
	}
	for _, problem := range p.Problems {
		if mapping.Problems[problem.Label] != problem.ProblemID {
			return fmt.Errorf("请确认题号 %s 对应题目 #%d", problem.Label, problem.ProblemID)
		}
	}
	return nil
}
