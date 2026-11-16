package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"nexcode/internal/auth"
	"nexcode/internal/engine"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type limitedBuffer struct {
	bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.max {
		return 0, errors.New("runner output limit exceeded")
	}
	return b.Buffer.Write(p)
}
func main() {
	token := os.Getenv("RUNNER_TOKEN")
	if len(token) < 32 {
		log.Fatal("RUNNER_TOKEN must contain at least 32 characters")
	}
	image := os.Getenv("SANDBOX_IMAGE")
	if image == "" {
		image = "nexcode-sandbox:local"
	}
	slots := make(chan struct{}, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "docker", "image", "inspect", image).Run(); err != nil {
			http.Error(w, "sandbox image unavailable", 503)
			return
		}
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /execute", func(w http.ResponseWriter, r *http.Request) {
		supplied := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(supplied), []byte(token)) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "runner busy", 503)
			return
		}
		var v engine.Request
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		d.DisallowUnknownFields()
		if err := d.Decode(&v); err != nil {
			http.Error(w, "invalid payload", 400)
			return
		}
		if len(v.Code) == 0 || len(v.Code) > 65536 || len(v.Runs) == 0 || len(v.Runs) > 20 || v.MemoryMB < 64 || v.MemoryMB > 256 {
			http.Error(w, "invalid limits", 400)
			return
		}
		budget := 20 * time.Second
		for _, run := range v.Runs {
			if run.TimeoutMS < 1 || run.TimeoutMS > 5000 || len(run.Stdin) > 32768 {
				http.Error(w, "invalid run", 400)
				return
			}
			budget += time.Duration(run.TimeoutMS) * time.Millisecond
		}
		payload, _ := json.Marshal(v)
		name := "nexcode-" + auth.Random()[:20]
		ctx, cancel := context.WithTimeout(r.Context(), budget)
		defer cancel()
		defer func() {
			cleanup, c := context.WithTimeout(context.Background(), 5*time.Second)
			defer c()
			exec.CommandContext(cleanup, "docker", "rm", "-f", name).Run()
		}()
		args := []string{"run", "--rm", "--name", name, "--pull=never", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges:true", "--pids-limit=32", "--memory=384m", "--memory-swap=384m", "--cpus=1", "--ulimit=nofile=64:64", "--ulimit=core=0:0", "--user=65534:65534", "--init", "-i", image}
		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Stdin = bytes.NewReader(payload)
		out := &limitedBuffer{max: 8 << 20}
		stderr := &limitedBuffer{max: 16384}
		cmd.Stdout = out
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			log.Printf("sandbox failure: %v: %s", err, stderr.String())
			http.Error(w, "sandbox execution unavailable", 503)
			return
		}
		var result engine.Response
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			log.Printf("invalid sandbox response: %v", err)
			http.Error(w, "invalid sandbox response", 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, bytes.NewReader(out.Bytes()))
	})
	server := &http.Server{Addr: ":8090", Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 30 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	log.Print("Isolated runner listening on :8090")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
