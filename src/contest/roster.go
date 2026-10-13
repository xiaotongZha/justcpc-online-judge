package contest

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// rosterEntry 是一行选手名单。password 只在导出考试包时使用，OJ 数据库不保存。
type rosterEntry struct {
	StudentID string
	Name      string
	Password  string
}

// ParseRoster 解析管理员上传的名单，支持 XLSX 与 CSV，CSV 做编码检测
// （UTF-8 / UTF-8 BOM / GBK），无法识别时返回明确错误。
// 表头固定为 student_id,name[,password]。
func ParseRoster(filename string, data []byte) ([]rosterEntry, error) {
	if len(data) == 0 {
		return nil, errors.New("名单文件为空")
	}
	var rows [][]string
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) || strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		parsed, err := readXLSX(data)
		if err != nil {
			return nil, err
		}
		rows = parsed
	} else {
		text, err := decodeText(data)
		if err != nil {
			return nil, err
		}
		parsed, err := csv.NewReader(strings.NewReader(text)).ReadAll()
		if err != nil {
			return nil, fmt.Errorf("名单格式错误：%w", err)
		}
		rows = parsed
	}
	return normalizeRoster(rows)
}

func readXLSX(data []byte) ([][]string, error) {
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("无法读取 XLSX：%w", err)
	}
	defer file.Close()
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("XLSX 中没有工作表")
	}
	rows, err := file.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("读取 XLSX 工作表失败：%w", err)
	}
	return rows, nil
}

// decodeText 检测文本编码：UTF-8 BOM / UTF-8 / GBK，无法识别时报错。
func decodeText(data []byte) (string, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(data) {
		return string(data), nil
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(data)
	if err != nil || !utf8.Valid(decoded) {
		return "", errors.New("无法识别名单文件编码，请使用 UTF-8 或 GBK 编码")
	}
	if bytes.ContainsRune(decoded, '\uFFFD') {
		return "", errors.New("无法识别名单文件编码，请使用 UTF-8 或 GBK 编码")
	}
	return string(decoded), nil
}

func normalizeRoster(rows [][]string) ([]rosterEntry, error) {
	// 去掉开头的空行
	start := 0
	for start < len(rows) && isBlankRow(rows[start]) {
		start++
	}
	if start >= len(rows) {
		return nil, errors.New("名单为空")
	}
	header := trimRow(rows[start])
	if len(header) < 2 || header[0] != "student_id" || header[1] != "name" {
		return nil, errors.New("名单表头必须是 student_id,name[,password]")
	}
	hasPassword := len(header) >= 3 && header[2] == "password"
	entries := make([]rosterEntry, 0, len(rows)-start-1)
	seen := map[string]bool{}
	for i := start + 1; i < len(rows); i++ {
		row := trimRow(rows[i])
		if isBlankRow(row) {
			continue
		}
		if len(row) < 2 || row[0] == "" || row[1] == "" {
			return nil, fmt.Errorf("第 %d 行缺少学号或姓名", i+1)
		}
		if seen[row[0]] {
			return nil, fmt.Errorf("第 %d 行学号 %s 重复", i+1, row[0])
		}
		seen[row[0]] = true
		entry := rosterEntry{StudentID: row[0], Name: row[1]}
		if hasPassword && len(row) >= 3 {
			entry.Password = row[2]
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, errors.New("名单没有有效数据行")
	}
	return entries, nil
}

func trimRow(row []string) []string {
	result := make([]string, len(row))
	for i, value := range row {
		result[i] = strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
	}
	return result
}

func isBlankRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

// RosterCSV 生成给考试系统使用的名单（含初始密码），UTF-8 BOM。
func RosterCSV(entries []rosterEntry) []byte {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xef, 0xbb, 0xbf})
	writer := csv.NewWriter(&buffer)
	_ = writer.Write([]string{"student_id", "name", "password"})
	for _, entry := range entries {
		password := entry.Password
		if password == "" {
			password = entry.StudentID
		}
		_ = writer.Write([]string{entry.StudentID, entry.Name, password})
	}
	writer.Flush()
	return buffer.Bytes()
}

// RosterExampleCSV 返回名单示例（CSV，UTF-8 BOM）。
func RosterExampleCSV() []byte {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xef, 0xbb, 0xbf})
	writer := csv.NewWriter(&buffer)
	for _, row := range rosterExampleRows() {
		_ = writer.Write(row)
	}
	writer.Flush()
	return buffer.Bytes()
}

// RosterExampleXLSX 返回名单示例（XLSX）。
func RosterExampleXLSX() ([]byte, error) {
	file := excelize.NewFile()
	sheet := "名单"
	file.SetSheetName("Sheet1", sheet)
	for r, row := range rosterExampleRows() {
		for c, value := range row {
			cell, err := excelize.CoordinatesToCellName(c+1, r+1)
			if err != nil {
				return nil, err
			}
			if err := file.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func rosterExampleRows() [][]string {
	return [][]string{
		{"student_id", "name", "password"},
		{"20260001", "张三", "pw0001"},
		{"20260002", "李四", "pw0002"},
	}
}
