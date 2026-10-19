package web

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"justcpc-online-judge/src/problem"
)

const testdataDownloadInterval = 5 * time.Second

type testdataDownloadKey struct {
	accountID, problemID int64
	caseID, kind         string
}

type testdataDownloadLimiter struct {
	mu          sync.Mutex
	until       map[testdataDownloadKey]time.Time
	lastCleanup time.Time
}

func (l *testdataDownloadLimiter) allow(key testdataDownloadKey) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.until == nil {
		l.until = make(map[testdataDownloadKey]time.Time)
	}
	if now.Sub(l.lastCleanup) >= testdataDownloadInterval {
		for key, until := range l.until {
			if !now.Before(until) {
				delete(l.until, key)
			}
		}
		l.lastCleanup = now
	}
	if until := l.until[key]; now.Before(until) {
		return false, int((until.Sub(now) + time.Second - 1) / time.Second)
	}
	l.until[key] = now.Add(testdataDownloadInterval)
	return true, 0
}

func (s Server) downloadTestdataFile(c *gin.Context) {
	u := s.user(c)
	id := number(c, "id")
	if c.IsAborted() {
		return
	}
	p, err := s.Problems.Get(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	if u.Role != "admin" && (p.OwnerAccountID == nil || *p.OwnerAccountID != u.ID) {
		fail(c, problem.ErrForbidden)
		return
	}
	caseID, kind := c.Param("case"), c.Param("kind")
	n, err := strconv.Atoi(caseID)
	if err != nil || n < 1 || strconv.Itoa(n) != caseID || (kind != "input" && kind != "output") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "测试点或文件类型无效"})
		return
	}
	if p.TestdataPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "测试数据不存在"})
		return
	}
	extension := ".in"
	if kind == "output" {
		extension = ".out"
	}
	name := caseID + extension
	path := s.Store.Absolute(filepath.Join(p.TestdataPath, name))
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "测试数据不存在"})
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	key := testdataDownloadKey{u.ID, p.ID, caseID, kind}
	if ok, retryAfter := s.testdataDownloads.allow(key); !ok {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "请勿触发重复下载，请五秒后再试"})
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.FileAttachment(path, name)
}
