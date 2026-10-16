package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"justcpc-online-judge/src/account"
	"justcpc-online-judge/src/contest"
	"justcpc-online-judge/src/judge"
	"justcpc-online-judge/src/problem"
	"justcpc-online-judge/src/store"
	"justcpc-online-judge/src/submission"
	"justcpc-online-judge/src/web"
)

// config 汇集启动所需配置。除 session_secret 与 initial_admin_password 外的字段
// 只从配置文件读取；缺失的配置项使用内置默认值。两个密文字段允许由同名环境变量
// 补充，配置文件不填写时回退到环境变量。
type config struct {
	Data                 string `json:"data"`
	Listen               string `json:"listen"`
	JudgeURL             string `json:"judge_url"`
	JudgeWorkers         int    `json:"judge_workers"`
	WebDist              string `json:"web_dist"`
	SessionSecret        string `json:"session_secret"`
	InitialAdminPassword string `json:"initial_admin_password"`
}

func defaultConfig() config {
	return config{
		Data:         "./data",
		Listen:       "127.0.0.1:18081",
		JudgeURL:     "http://127.0.0.1:18080",
		JudgeWorkers: 0,
		WebDist:      "./web/dist",
	}
}

// loadConfig 首次运行生成配置；已有配置中的空密文允许由环境变量补充。
func loadConfig(path string) (config, error) {
	cfg := defaultConfig()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return generateConfig(path)
		}
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	if cfg.JudgeWorkers < 0 {
		return cfg, errors.New("judge_workers must be non-negative (0 means automatic)")
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = os.Getenv("JUSTCPC_SESSION_SECRET")
	}
	if cfg.InitialAdminPassword == "" {
		cfg.InitialAdminPassword = os.Getenv("JUSTCPC_INITIAL_ADMIN_PASSWORD")
	}
	return cfg, nil
}

func generateConfig(path string) (config, error) {
	cfg := defaultConfig()
	cfg.Listen = "0.0.0.0:18081"
	cfg.SessionSecret = os.Getenv("JUSTCPC_SESSION_SECRET")
	cfg.InitialAdminPassword = os.Getenv("JUSTCPC_INITIAL_ADMIN_PASSWORD")
	var random [50]byte
	if _, err := rand.Read(random[:]); err != nil {
		return cfg, err
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = base64.RawURLEncoding.EncodeToString(random[:32])
	}
	if cfg.InitialAdminPassword == "" {
		cfg.InitialAdminPassword = base64.RawURLEncoding.EncodeToString(random[32:])
	}
	if len(cfg.SessionSecret) < 32 {
		return cfg, errors.New("JUSTCPC_SESSION_SECRET must contain at least 32 bytes")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return cfg, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return loadConfig(path)
	}
	if err != nil {
		return cfg, err
	}
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return cfg, err
	}
	log.Printf("已生成配置 %s；初始管理员：admin / %s（首次登录后请修改密码）", path, cfg.InitialAdminPassword)
	return cfg, nil
}

func main() {
	configPath := os.Getenv("JUSTCPC_CONFIG")
	if configPath == "" {
		configPath = "config.json"
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("load configuration %s: %v", configPath, err)
	}
	if len(cfg.SessionSecret) < 32 {
		log.Fatalf("session_secret must be set in %s (or JUSTCPC_SESSION_SECRET) and contain at least 32 bytes", configPath)
	}
	storage, err := store.Open(filepath.Join(cfg.Data, "justcpc.sqlite"), cfg.Data)
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()
	if err := storage.EnsureLayout(); err != nil {
		log.Fatal(err)
	}
	accounts := account.Service{Store: storage}
	if err := accounts.EnsureAdmin(context.Background(), "admin", cfg.InitialAdminPassword); err != nil {
		log.Fatal(err)
	}
	sessions, err := account.NewSessionManager(cfg.SessionSecret)
	if err != nil {
		log.Fatal(err)
	}
	problems := problem.Service{Store: storage}
	submissions := submission.Service{Store: storage, Problems: problems}
	dispatcher := judge.Dispatcher{Store: storage, Submissions: submissions, Problems: problems, Client: judge.Client{URL: cfg.JudgeURL}, Workers: cfg.JudgeWorkers}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		if err := dispatcher.Run(ctx); err != nil {
			log.Printf("judge dispatcher stopped: %v", err)
		}
	}()
	server := web.Server{Accounts: accounts, Sessions: sessions, Problems: problems, Submissions: submissions, Contests: contest.Service{Store: storage}, Store: storage, Dispatcher: &dispatcher, WebDist: cfg.WebDist}
	httpServer := &http.Server{Addr: cfg.Listen, Handler: server.Handler()}
	go func() {
		log.Printf("JustCPC OJ listening on %s (config %s; judge_workers=%d, 0=auto)", cfg.Listen, configPath, cfg.JudgeWorkers)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Printf("JustCPC OJ stopped")
}
