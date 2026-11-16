package engine

import (
	"context"
	"errors"
	"fmt"
	"nexcode/internal/store"
	"path/filepath"
	"testing"
	"time"
)

type fileExecutor struct{ mode string }

func (f fileExecutor) Execute(_ context.Context, request Request) (Response, error) {
	if request.Runs[0].Files["input.txt"] != "source" {
		return Response{}, errors.New("input file missing")
	}
	files := map[string]string{}
	if f.mode != "missing" {
		files["report.txt"] = "expected\n"
	}
	if f.mode == "wrong" {
		files["report.txt"] = "wrong\n"
	}
	return Response{Results: []Result{{Stdout: "", ExitCode: 0, Files: files}}}, nil
}

func TestQueueRequiresExpectedFilesToExistAndMatch(t *testing.T) {
	for _, mode := range []string{"correct", "missing", "wrong"} {
		t.Run(mode, func(t *testing.T) {
			s, err := store.Open(filepath.Join(t.TempDir(), "files.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.DB.Close()
			if err = s.Seed(); err != nil {
				t.Fatal(err)
			}
			s.DB.Exec(`INSERT INTO users(id,full_name,group_name,role) VALUES(1,'FILES','TEST','student')`)
			es, _ := s.Exercises(1)
			exercise := es[0]
			exercise.Tests = `{"stdin":"","stdout":"","files":{"input.txt":"source"},"expected_files":{"report.txt":"expected\n"}}`
			result, err := s.DB.Exec(`INSERT INTO submissions(user_id,exercise_id,code,mode,status) VALUES(1,?,'file code','check','queued')`, exercise.ID)
			if err != nil {
				t.Fatal(err)
			}
			sid, _ := result.LastInsertId()
			queue := NewQueue(s, fileExecutor{mode}, 1, 2)
			defer queue.Close()
			queue.Submit(Job{ID: sid, Exercise: exercise, Code: "file code", Mode: "check"})
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				var status string
				s.DB.QueryRow(`SELECT status FROM submissions WHERE id=?`, sid).Scan(&status)
				if status != "queued" && status != "running" {
					want := "failed"
					if mode == "correct" {
						want = "passed"
					}
					if status != want {
						t.Fatalf("%s: %s", mode, status)
					}
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
			t.Fatal("job did not finish")
		})
	}
}

func TestFileTestValidationRejectsHostPaths(t *testing.T) {
	for _, name := range []string{"../secret", "/etc/passwd", "C:\\file", "dir/file"} {
		text := fmt.Sprintf(`{"stdin":"","stdout":"","files":{%q:"value"}}`, name)
		if _, err := ParseTests(text); err == nil {
			t.Fatalf("host path accepted: %s", name)
		}
	}
}
