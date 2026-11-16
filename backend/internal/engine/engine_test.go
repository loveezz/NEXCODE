package engine

import (
	"context"
	"encoding/json"
	"nexcode/internal/store"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTests(t *testing.T) {
	cases := []struct {
		name, text string
		valid      bool
	}{
		{"unicode and blanks", "\n{\"stdin\":\"Анна\\n\",\"stdout\":\"Привет!\\n\"}\n", true},
		{"empty", "", false}, {"missing expected", `{"stdin":""}`, false}, {"null stdin", `{"stdin":null,"stdout":""}`, false},
		{"wrong type", `{"stdin":2,"stdout":""}`, false}, {"unknown field", `{"stdin":"","stdout":"","command":"rm"}`, false},
		{"invalid mode", `{"stdin":"","stdout":"","compare":"regex"}`, false}, {"timeout", `{"stdin":"","stdout":"","timeout_ms":5001}`, false},
		{"too many", strings.Repeat("{\"stdin\":\"\",\"stdout\":\"\"}\n", 21), false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTests(tt.text)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}
func TestOutputComparison(t *testing.T) {
	if !Equal("  hi\r\n", "hi\n", "trim") {
		t.Fatal("trim/CRLF")
	}
	if Equal("hi\n", "hi", "exact") {
		t.Fatal("exact must preserve final newline")
	}
	if Equal("hello  world", "hello world", "trim") {
		t.Fatal("internal whitespace must be preserved")
	}
}

type fixtureExecutor struct{}

func (fixtureExecutor) Execute(ctx context.Context, r Request) (Response, error) {
	out := Response{Lint: []Diagnostic{}}
	for range r.Runs {
		out.Results = append(out.Results, Result{Stdout: "42\n", ExitCode: 0, DurationMS: 1})
	}
	return out, nil
}
func TestQueuePersistsExpectedAndActual(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	if err = s.Seed(); err != nil {
		t.Fatal(err)
	}
	s.DB.Exec(`INSERT INTO users(full_name,group_name,role) VALUES('TEST','TEST','student')`)
	es, err := s.Exercises(1)
	if err != nil {
		t.Fatal(err)
	}
	e := es[0]
	e.Tests = `{"stdin":"test","stdout":"42\n"}`
	res, err := s.DB.Exec(`INSERT INTO submissions(user_id,exercise_id,code,mode,status) VALUES(1,?,'print(42)','check','queued')`, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	sid, _ := res.LastInsertId()
	q := NewQueue(s, fixtureExecutor{}, 1, 2)
	defer q.Close()
	q.Submit(Job{ID: sid, Exercise: e, Code: "print(42)", Mode: "check"})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var status, raw string
		s.DB.QueryRow(`SELECT status,report FROM submissions WHERE id=?`, sid).Scan(&status, &raw)
		if status == "passed" {
			var r Report
			if err = json.Unmarshal([]byte(raw), &r); err != nil {
				t.Fatal(err)
			}
			if len(r.Results) != 1 || r.Results[0].Expected != "42\n" || r.Results[0].Actual != "42\n" || r.Results[0].Stdin != "test" {
				t.Fatalf("wire report: %s", raw)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("queue did not complete")
}
