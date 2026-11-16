package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"nexcode/internal/store"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Test struct {
	Stdin         string            `json:"stdin"`
	Stdout        string            `json:"stdout"`
	TimeoutMS     int               `json:"timeout_ms,omitempty"`
	Compare       string            `json:"compare,omitempty"`
	Files         map[string]string `json:"files,omitempty"`
	ExpectedFiles map[string]string `json:"expected_files,omitempty"`
	DeniedFiles   []string          `json:"denied_files,omitempty"`
}
type Run struct {
	Stdin       string            `json:"stdin"`
	TimeoutMS   int               `json:"timeout_ms"`
	Files       map[string]string `json:"files,omitempty"`
	DeniedFiles []string          `json:"denied_files,omitempty"`
}
type Request struct {
	Code     string `json:"code"`
	Runs     []Run  `json:"runs"`
	MemoryMB int    `json:"memory_mb"`
}
type Diagnostic struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Location struct {
		Row    int `json:"row"`
		Column int `json:"column"`
	} `json:"location"`
}
type Result struct {
	Stdout      string            `json:"stdout"`
	Stderr      string            `json:"stderr"`
	ExitCode    int               `json:"exit_code"`
	DurationMS  float64           `json:"duration_ms"`
	TimedOut    bool              `json:"timed_out"`
	OutputLimit bool              `json:"output_limit"`
	Files       map[string]string `json:"files,omitempty"`
}
type Response struct {
	Results []Result     `json:"results"`
	Lint    []Diagnostic `json:"lint"`
}
type TestResult struct {
	Result
	Stdin         string            `json:"stdin"`
	Expected      string            `json:"expected"`
	Actual        string            `json:"actual"`
	Passed        bool              `json:"passed"`
	InputFiles    map[string]string `json:"input_files,omitempty"`
	ExpectedFiles map[string]string `json:"expected_files,omitempty"`
}
type Report struct {
	Results  []TestResult `json:"results"`
	Lint     []Diagnostic `json:"lint"`
	Passed   int          `json:"passed"`
	Total    int          `json:"total"`
	Error    string       `json:"error,omitempty"`
	Review   string       `json:"review,omitempty"`
	Feedback string       `json:"feedback,omitempty"`
}

// Only simple relative virtual filenames are accepted. No host paths or directories.
func ValidateFiles(files map[string]string, denied []string) error {
	if len(files) > 16 || len(denied) > 16 {
		return errors.New("Не более 16 учебных файлов")
	}
	total := 0
	valid := func(name string) bool {
		if len(name) == 0 || len(name) > 100 || name == "." || name == ".." {
			return false
		}
		for _, c := range name {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
				return false
			}
		}
		return true
	}
	for name, content := range files {
		if !valid(name) || len(content) > 32768 {
			return errors.New("Недопустимое имя или размер учебного файла")
		}
		total += len(content)
	}
	for _, name := range denied {
		if !valid(name) {
			return errors.New("Недопустимое имя учебного файла")
		}
	}
	if total > 65536 {
		return errors.New("Учебные файлы превышают 64 КБ")
	}
	return nil
}

type Executor interface {
	Execute(context.Context, Request) (Response, error)
}
type Remote struct{ URL, Token string }

func (r Remote) Execute(ctx context.Context, v Request) (Response, error) {
	b, _ := json.Marshal(v)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(r.URL, "/")+"/execute", bytes.NewReader(b))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Response{}, errors.New("Сервис исполнения недоступен")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Response{}, fmt.Errorf("Сервис исполнения вернул код %d", resp.StatusCode)
	}
	var out Response
	err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&out)
	return out, err
}
func ParseTests(text string) ([]Test, error) {
	if len(text) > 1<<20 {
		return nil, errors.New("Набор тестов превышает 1 МБ")
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	tests := []Test{}
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, fmt.Errorf("Строка %d: некорректный JSON", line)
		}
		if fields["stdin"] == nil || fields["stdout"] == nil || string(fields["stdin"]) == "null" || string(fields["stdout"]) == "null" {
			return nil, fmt.Errorf("Строка %d: обязательны строки stdin и stdout", line)
		}
		var t Test
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&t); err != nil {
			return nil, fmt.Errorf("Строка %d: %w", line, err)
		}
		if t.TimeoutMS < 0 || t.TimeoutMS > 5000 {
			return nil, errors.New("timeout_ms: от 1 до 5000, либо 0 для значения упражнения")
		}
		if t.Compare != "" && t.Compare != "trim" && t.Compare != "exact" {
			return nil, errors.New("compare: exact или trim")
		}
		if len(t.Stdin) > 32768 || len(t.Stdout) > 32768 {
			return nil, errors.New("Вход и вывод теста ограничены 32 КБ")
		}
		if err := ValidateFiles(t.Files, t.DeniedFiles); err != nil {
			return nil, err
		}
		if err := ValidateFiles(t.ExpectedFiles, nil); err != nil {
			return nil, err
		}
		tests = append(tests, t)
		if len(tests) > 20 {
			return nil, errors.New("Не более 20 тестов")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(tests) == 0 {
		return nil, errors.New("Добавьте хотя бы один тест")
	}
	return tests, nil
}
func Equal(actual, expected, mode string) bool {
	actual = strings.ReplaceAll(actual, "\r\n", "\n")
	expected = strings.ReplaceAll(expected, "\r\n", "\n")
	if mode != "exact" {
		actual = strings.TrimSpace(actual)
		expected = strings.TrimSpace(expected)
	}
	return actual == expected
}

type Job struct {
	ID                int64
	Exercise          store.Exercise
	Code, Mode, Stdin string
}
type Queue struct {
	Store    *store.Store
	Executor Executor
	jobs     chan Job
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	Active   atomic.Int64
}

func NewQueue(s *store.Store, e Executor, workers, capacity int) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	q := &Queue{Store: s, Executor: e, jobs: make(chan Job, capacity), ctx: ctx, cancel: cancel}
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker()
	}
	return q
}
func (q *Queue) Submit(j Job) bool {
	select {
	case q.jobs <- j:
		return true
	default:
		return false
	}
}
func (q *Queue) Pending() int { return len(q.jobs) }
func (q *Queue) Close()       { q.cancel(); q.wg.Wait() }
func (q *Queue) worker() {
	defer q.wg.Done()
	for {
		select {
		case <-q.ctx.Done():
			return
		case j := <-q.jobs:
			q.Active.Add(1)
			q.process(j)
			q.Active.Add(-1)
		}
	}
}
func (q *Queue) process(j Job) {
	q.Store.DB.Exec(`UPDATE submissions SET status='running' WHERE id=?`, j.ID)
	report := Report{Results: []TestResult{}, Lint: []Diagnostic{}}
	status := "error"
	defer func() {
		if recover() != nil {
			report.Error = "Ошибка обработчика исполнения"
		}
		b, _ := json.Marshal(report)
		q.Store.DB.Exec(`UPDATE submissions SET status=?,report=? WHERE id=?`, status, string(b), j.ID)
	}()
	tests := []Test{{Stdin: j.Stdin, TimeoutMS: j.Exercise.TimeoutMS}}
	if j.Mode == "check" {
		var err error
		tests, err = ParseTests(j.Exercise.Tests)
		if err != nil {
			report.Error = err.Error()
			return
		}
	} else if samples, err := ParseTests(j.Exercise.Tests); err == nil && len(samples) > 0 {
		tests[0].Files = samples[0].Files
		tests[0].DeniedFiles = samples[0].DeniedFiles
	}
	runs := []Run{}
	budget := 30 * time.Second
	for _, t := range tests {
		ms := t.TimeoutMS
		if ms == 0 {
			ms = j.Exercise.TimeoutMS
		}
		runs = append(runs, Run{Stdin: t.Stdin, TimeoutMS: ms, Files: t.Files, DeniedFiles: t.DeniedFiles})
		budget += time.Duration(ms) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(q.ctx, budget)
	defer cancel()
	out, err := q.Executor.Execute(ctx, Request{j.Code, runs, j.Exercise.MemoryMB})
	if err != nil {
		report.Error = err.Error()
		return
	}
	if len(out.Results) != len(tests) {
		report.Error = "Неполный ответ сервиса исполнения"
		return
	}
	report.Lint = out.Lint
	if report.Lint == nil {
		report.Lint = []Diagnostic{}
	}
	for i, r := range out.Results {
		passed := r.ExitCode == 0 && !r.TimedOut && !r.OutputLimit
		if j.Mode == "check" {
			passed = passed && Equal(r.Stdout, tests[i].Stdout, tests[i].Compare)
			for name, expected := range tests[i].ExpectedFiles {
				actual, exists := r.Files[name]
				passed = passed && exists && Equal(actual, expected, tests[i].Compare)
			}
		}
		report.Results = append(report.Results, TestResult{Result: r, Stdin: tests[i].Stdin, Expected: tests[i].Stdout, Actual: r.Stdout, Passed: passed, InputFiles: tests[i].Files, ExpectedFiles: tests[i].ExpectedFiles})
		if passed {
			report.Passed++
		}
	}
	report.Total = len(tests)
	status = "failed"
	if report.Passed == report.Total {
		status = "passed"
		if len(report.Lint) > 0 {
			status = "warnings"
		}
	}
	if j.Mode == "run" && report.Passed == report.Total {
		status = "executed"
	}
}
