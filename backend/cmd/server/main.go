package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"nexcode/internal/auth"
	"nexcode/internal/engine"
	"nexcode/internal/httpapi"
	"nexcode/internal/store"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func number(k string, d int) int {
	v, e := strconv.Atoi(os.Getenv(k))
	if e != nil || v < 1 || v > 128 {
		return d
	}
	return v
}
func main() {
	logPath := env("LOG_PATH", "logs/nexcode.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		log.Fatal(err)
	}
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.MultiWriter(logFile, os.Stdout), nil)))
	token := os.Getenv("RUNNER_TOKEN")
	if len(token) < 32 {
		log.Fatal("RUNNER_TOKEN must contain at least 32 characters")
	}
	s, err := store.Open(env("DB_PATH", "data/nexcode.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer s.DB.Close()
	if env("SEED_PRACTICE", "true") == "true" {
		if err = s.Seed(); err != nil {
			log.Fatal(err)
		}
	}
	location, err := time.LoadLocation(env("APP_TIMEZONE", "Asia/Yekaterinburg"))
	if err != nil {
		log.Fatal(err)
	}
	appURL := env("APP_URL", "http://localhost:3000")
	a := &auth.Service{Store: s, Secure: env("COOKIE_SECURE", "false") == "true" || strings.HasPrefix(appURL, "https://"), Logger: slog.Default(), Location: location}
	if err = a.InitSemester(os.Getenv("SEMESTER_END")); err != nil {
		log.Fatal(err)
	}
	if hash := os.Getenv("ADMIN_PASSWORD_HASH"); hash != "" {
		data, _ := json.Marshal([]auth.StaffAccount{{Login: env("ADMIN_LOGIN", "admin"), FullName: "Администратор", Role: "admin", PasswordHash: hash}})
		if err = a.ConfigureStaff(data); err != nil {
			log.Fatal(err)
		}
	}
	if configPath := os.Getenv("STAFF_ACCOUNTS_FILE"); configPath != "" {
		data, er := os.ReadFile(configPath)
		if er != nil {
			log.Fatal("Cannot read staff accounts configuration")
		}
		if er = a.ConfigureStaff(data); er != nil {
			log.Fatal(er)
		}
	}
	var adminCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin' AND blocked=0`).Scan(&adminCount)
	if adminCount == 0 {
		log.Fatal("Configure an administrator using ADMIN_PASSWORD_HASH or STAFF_ACCOUNTS_FILE")
	}
	q := engine.NewQueue(s, engine.Remote{URL: env("RUNNER_URL", "http://127.0.0.1:8090"), Token: token}, number("WORKERS", 2), number("QUEUE_CAPACITY", 64))
	server := &http.Server{Addr: env("LISTEN_ADDR", "127.0.0.1:8080"), Handler: httpapi.New(s, a, q, appURL), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	stop := make(chan os.Signal, 1)
	stopped := make(chan struct{})
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
		q.Close()
		close(stopped)
	}()
	log.Printf("NEXCODE API listening on %s", server.Addr)
	if err = server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-stopped
}
