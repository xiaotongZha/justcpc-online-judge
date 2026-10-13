package web

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"justcpc-online-judge/src/account"
	"justcpc-online-judge/src/contest"
	"justcpc-online-judge/src/judge"
	"justcpc-online-judge/src/problem"
	"justcpc-online-judge/src/store"
	"justcpc-online-judge/src/submission"
)

type Server struct {
	Accounts    account.Service
	Sessions    *account.SessionManager
	Problems    problem.Service
	Submissions submission.Service
	Contests    contest.Service
	Store       *store.Store
	Dispatcher  *judge.Dispatcher
	WebDist     string
}

type principal struct {
	ID   int64
	Role string
}

const principalKey = "principal"

func (s Server) Handler() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	api := router.Group("/api")
	api.POST("/login", s.login)
	api.POST("/logout", s.logout)
	auth := api.Group("")
	auth.Use(s.requireLogin)
	auth.GET("/me", s.me)
	auth.GET("/accounts", s.adminOnly, s.listAccounts)
	auth.POST("/accounts", s.adminOnly, s.createAccount)
	auth.PUT("/accounts/:id", s.adminOnly, s.updateAccount)
	auth.POST("/admin/judge-queue/retry", s.adminOnly, s.retryFailedQueue)
	auth.GET("/admin/judge-queue", s.adminOnly, s.judgeQueueStatus)
	auth.GET("/problems", s.listProblems)
	auth.GET("/problems/:id/editor", s.getProblemEditor)
	auth.GET("/problems/:id", s.getProblem)
	auth.POST("/problems", s.createProblem)
	auth.PUT("/problems/:id", s.updateProblem)
	auth.DELETE("/problems/:id", s.deleteProblem)
	auth.POST("/problems/:id/testdata", s.uploadTestdata)
	auth.POST("/problems/:id/submissions", s.createSubmission)
	auth.GET("/submissions", s.listSubmissions)
	auth.GET("/submissions/:id", s.getSubmission)
	auth.GET("/contests", s.listContests)
	auth.POST("/contests", s.adminOnly, s.createContest)
	auth.DELETE("/contests/:id", s.adminOnly, s.deleteContest)
	auth.GET("/contest-roster-example.csv", s.rosterExampleCSV)
	auth.GET("/contest-roster-example.xlsx", s.rosterExampleXLSX)
	auth.GET("/contests/:id", s.getContest)
	auth.GET("/contests/:id/statement.pdf", s.contestStatementPDF)
	auth.POST("/contests/:id/export-exam", s.adminOnly, s.exportExam)
	auth.POST("/contests/:id/import-submissions", s.adminOnly, s.importSubmissions)
	auth.GET("/contests/:id/evaluation", s.contestEvaluation)
	auth.POST("/contests/:id/enqueue", s.adminOnly, s.enqueueContest)
	auth.GET("/contests/:id/scoreboard.csv", s.scoreboardCSV)
	auth.GET("/contests/:id/result-package", s.resultPackage)
	if info, err := os.Stat(s.WebDist); err == nil && info.IsDir() {
		router.StaticFS("/assets", http.Dir(filepath.Join(s.WebDist, "assets")))
		router.NoRoute(func(c *gin.Context) {
			if c.Request.Method == http.MethodGet {
				c.File(filepath.Join(s.WebDist, "index.html"))
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		})
	}
	return router
}

func (s Server) requireLogin(c *gin.Context) {
	id, role, ok := s.Sessions.Parse(c.Request)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}
	c.Set(principalKey, principal{id, role})
	c.Next()
}
func (s Server) adminOnly(c *gin.Context) {
	if s.user(c).Role != "admin" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "administrator permission required"})
		return
	}
	c.Next()
}
func (s Server) user(c *gin.Context) principal {
	value, _ := c.Get(principalKey)
	return value.(principal)
}
func (s Server) login(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &input) {
		return
	}
	a, err := s.Accounts.Authenticate(c, input.Username, input.Password)
	if err != nil {
		fail(c, err)
		return
	}
	s.Sessions.Set(c.Writer, a)
	c.JSON(http.StatusOK, a)
}
func (s Server) logout(c *gin.Context) { s.Sessions.Clear(c.Writer); c.Status(http.StatusNoContent) }
func (s Server) me(c *gin.Context) {
	a, err := s.Accounts.Get(c, s.user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, a)
}
func (s Server) listAccounts(c *gin.Context) {
	items, err := s.Accounts.ListForAdmin(c)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (s Server) createAccount(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !bindJSON(c, &in) {
		return
	}
	item, err := s.Accounts.Create(c, in.Username, in.Password, in.Role)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (s Server) updateAccount(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !bindJSON(c, &in) {
		return
	}
	item, err := s.Accounts.Update(c, number(c, "id"), in.Username, in.Password, in.Role)
	if err != nil {
		fail(c, err)
		return
	}
	if s.user(c).ID == item.ID {
		s.Sessions.Set(c.Writer, item)
	}
	c.JSON(http.StatusOK, item)
}
func (s Server) retryFailedQueue(c *gin.Context) {
	if s.Dispatcher == nil {
		fail(c, errors.New("judge dispatcher is unavailable"))
		return
	}
	count, err := s.Dispatcher.RetryFailed(c)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"retried": count})
}
func (s Server) judgeQueueStatus(c *gin.Context) {
	if s.Dispatcher == nil {
		fail(c, errors.New("judge dispatcher is unavailable"))
		return
	}
	items, err := s.Dispatcher.QueueStatus(c)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (s Server) listProblems(c *gin.Context) {
	items, err := s.Problems.ListPublic(c, s.user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (s Server) getProblem(c *gin.Context) {
	item, err := s.Problems.Public(c, number(c, "id"), s.user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s Server) getProblemEditor(c *gin.Context) {
	item, err := s.Problems.Editor(c, number(c, "id"), s.user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s Server) createProblem(c *gin.Context) {
	var in problem.CreateInput
	if !bindJSON(c, &in) {
		return
	}
	item, err := s.Problems.Create(c, s.user(c).ID, in)
	if err != nil {
		fail(c, err)
		return
	}
	editor, err := s.Problems.Editor(c, item.ID, s.user(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, editor)
}
func (s Server) updateProblem(c *gin.Context) {
	var in problem.UpdateInput
	if !bindJSON(c, &in) {
		return
	}
	u := s.user(c)
	item, err := s.Problems.Update(c, number(c, "id"), u.ID, in)
	if err != nil {
		fail(c, err)
		return
	}
	editor, err := s.Problems.Editor(c, item.ID, u.ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, editor)
}
func (s Server) deleteProblem(c *gin.Context) {
	u := s.user(c)
	if err := s.Problems.Delete(c, number(c, "id"), u.ID, u.Role == "admin"); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s Server) uploadTestdata(c *gin.Context) {
	u := s.user(c)
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, errors.New("testdata ZIP file is required"))
		return
	}
	content, err := file.Open()
	if err != nil {
		fail(c, err)
		return
	}
	defer content.Close()
	data, err := readLimited(content, 1<<30)
	if err != nil {
		fail(c, err)
		return
	}
	if err = s.Problems.UploadTestdata(c, number(c, "id"), u.ID, data); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s Server) createSubmission(c *gin.Context) {
	var in submission.CreateInput
	if !bindJSON(c, &in) {
		return
	}
	item, err := s.Submissions.Create(c, number(c, "id"), s.user(c).ID, in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (s Server) listSubmissions(c *gin.Context) {
	query := submission.SubmissionQuery{
		Status:   c.Query("status"),
		Language: c.Query("language"),
		Keyword:  c.Query("keyword"),
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "page_size", 20),
	}
	if value := c.Query("problem_id"); value != "" {
		query.ProblemID, _ = strconv.ParseInt(value, 10, 64)
	}
	page, err := s.Submissions.Page(c, query)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return fallback
	}
	return value
}
func (s Server) getSubmission(c *gin.Context) {
	u := s.user(c)
	item, err := s.Submissions.Detail(c, number(c, "id"), u.ID, u.Role == "admin")
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s Server) listContests(c *gin.Context) {
	items, err := s.Contests.List(c)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
func (s Server) rosterExampleCSV(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="roster-example.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", contest.RosterExampleCSV())
}
func (s Server) rosterExampleXLSX(c *gin.Context) {
	data, err := contest.RosterExampleXLSX()
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", `attachment; filename="roster-example.xlsx"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}
func (s Server) getContest(c *gin.Context) {
	detail, err := s.Contests.Detail(c, number(c, "id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}
func (s Server) contestStatementPDF(c *gin.Context) {
	path, err := s.Contests.StatementPDF(c, number(c, "id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.File(path)
}
func (s Server) createContest(c *gin.Context) {
	var input contest.CreateInput
	if err := json.Unmarshal([]byte(c.PostForm("payload")), &input); err != nil {
		fail(c, errors.New("比赛数据格式错误："+err.Error()))
		return
	}
	statement, err := formFileBytes(c, "statement")
	if err != nil {
		fail(c, errors.New("必须上传比赛题面 PDF"))
		return
	}
	rosterName, rosterData, err := formFile(c, "roster")
	if err != nil {
		fail(c, errors.New("必须上传名单文件（xlsx 或 csv）"))
		return
	}
	item, err := s.Contests.Create(c, s.user(c).ID, input, statement, rosterName, rosterData)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (s Server) deleteContest(c *gin.Context) {
	if err := s.Contests.Delete(c, s.user(c).ID, number(c, "id")); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s Server) exportExam(c *gin.Context) {
	id := number(c, "id")
	if c.IsAborted() {
		return
	}
	name, data, err := s.Contests.ExportExamPackage(c, s.user(c).ID, id)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
	c.Data(http.StatusOK, "application/zip", data)
}
func (s Server) importSubmissions(c *gin.Context) {
	id := number(c, "id")
	if c.IsAborted() {
		return
	}
	_, data, err := formFile(c, "file")
	if err != nil {
		fail(c, errors.New("必须上传考试系统导出的提交包"))
		return
	}
	result, err := s.Contests.ImportSubmissions(c, s.user(c).ID, id, data)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (s Server) contestEvaluation(c *gin.Context) {
	item, err := s.Contests.Evaluation(c, number(c, "id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s Server) enqueueContest(c *gin.Context) {
	var in struct {
		SubmissionIDs []int64 `json:"submission_ids"`
	}
	if !bindJSON(c, &in) {
		return
	}
	count, err := s.Contests.Enqueue(c, number(c, "id"), in.SubmissionIDs)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"enqueued": count})
}
func (s Server) scoreboardCSV(c *gin.Context) {
	board, err := s.Contests.LiveScoreboard(c, number(c, "id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=contest-%d-scoreboard.csv", board.ContestID))
	c.Writer.Write([]byte{0xef, 0xbb, 0xbf})
	header := []string{"排名", "学号", "姓名"}
	for _, p := range board.Problems {
		header = append(header, p.Label)
	}
	header = append(header, "总分")
	writer := csvWriter{c.Writer}
	writer.write(header)
	for _, row := range board.Rows {
		rank := strconv.Itoa(row.Rank)
		if !row.HasScore {
			rank = "-"
		}
		values := []string{rank, row.StudentID, row.Name}
		values = append(values, row.Scores...)
		values = append(values, row.Total)
		writer.write(values)
	}
}

func (s Server) resultPackage(c *gin.Context) {
	id := number(c, "id")
	if c.IsAborted() {
		return
	}
	data, err := s.Contests.ResultPackage(c, id)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=contest-%d-result.zip", id))
	c.Data(http.StatusOK, "application/zip", data)
}

type csvWriter struct{ w http.ResponseWriter }

func (w csvWriter) write(row []string) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	_ = writer.Write(row)
	writer.Flush()
	_, _ = w.w.Write(buffer.Bytes())
}
func bindJSON(c *gin.Context, value any) bool {
	if err := c.ShouldBindJSON(value); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}
func number(c *gin.Context, key string) int64 {
	value, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || value < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid identifier"})
		return 0
	}
	return value
}
func fail(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, sql.ErrNoRows) {
		status = http.StatusNotFound
	} else if errors.Is(err, problem.ErrForbidden) || errors.Is(err, submission.ErrForbidden) {
		status = http.StatusForbidden
	} else {
		status = http.StatusBadRequest
	}
	c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
}
func readLimited(value io.Reader, maximum int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(value, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, errors.New("uploaded file is too large")
	}
	return data, nil
}

func formFile(c *gin.Context, field string) (string, []byte, error) {
	header, err := c.FormFile(field)
	if err != nil {
		return "", nil, err
	}
	file, err := header.Open()
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	data, err := readLimited(file, 1<<30)
	if err != nil {
		return "", nil, err
	}
	return header.Filename, data, nil
}

func formFileBytes(c *gin.Context, field string) ([]byte, error) {
	_, data, err := formFile(c, field)
	return data, err
}
