package store

import (
	"database/sql"
	"encoding/json"
	_ "modernc.org/sqlite"
	"strings"
	"time"
)

type Store struct{ DB *sql.DB }
type User struct {
	ID           int64  `json:"id"`
	Login        string `json:"login"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Blocked      bool   `json:"blocked"`
	Created      string `json:"created_at"`
	FullName     string `json:"full_name"`
	GroupName    string `json:"group_name"`
	InviteCodeID *int64 `json:"invite_code_id"`
	LastLoginAt  string `json:"last_login_at"`
	LastLoginIP  string `json:"last_login_ip"`
}
type Lab struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Position    int    `json:"position"`
	Source      string `json:"source"`
	DocumentURL string `json:"document_url,omitempty"`
}
type Exercise struct {
	ID         int64  `json:"id"`
	LabID      int64  `json:"lab_id"`
	Title      string `json:"title"`
	Statement  string `json:"statement"`
	Template   string `json:"template"`
	Tests      string `json:"tests,omitempty"`
	TimeoutMS  int    `json:"timeout_ms"`
	MemoryMB   int    `json:"memory_mb"`
	Difficulty string `json:"difficulty"`
	Position   int    `json:"position"`
	Status     string `json:"status"`
	TestCount  int    `json:"test_count"`
	Kind       string `json:"kind"`
	CatalogKey string `json:"catalog_key,omitempty"`
}
type Submission struct {
	ID         int64           `json:"id"`
	UserID     int64           `json:"user_id"`
	ExerciseID int64           `json:"exercise_id"`
	Code       string          `json:"code"`
	Mode       string          `json:"mode"`
	Status     string          `json:"status"`
	Report     json.RawMessage `json:"report"`
	Created    string          `json:"created_at"`
	Title      string          `json:"title"`
	Login      string          `json:"login,omitempty"`
}

func (s *Store) Labs() ([]Lab, error) {
	rows, err := s.DB.Query(`SELECT id,title,description,position,source,document_url FROM labs WHERE archived=0 ORDER BY position,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Lab{}
	for rows.Next() {
		var l Lab
		if err = rows.Scan(&l.ID, &l.Title, &l.Description, &l.Position, &l.Source, &l.DocumentURL); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

const exerciseFields = `e.id,e.lab_id,e.title,e.statement,e.template,e.tests,e.timeout_ms,e.memory_mb,e.difficulty,e.position,e.kind,e.catalog_key`

func scanExercise(row interface{ Scan(...any) error }) (e Exercise, err error) {
	err = row.Scan(&e.ID, &e.LabID, &e.Title, &e.Statement, &e.Template, &e.Tests, &e.TimeoutMS, &e.MemoryMB, &e.Difficulty, &e.Position, &e.Kind, &e.CatalogKey)
	if e.Kind == "python" && strings.TrimSpace(e.Tests) != "" {
		e.TestCount = len(strings.Split(strings.TrimSpace(e.Tests), "\n"))
	}
	return
}
func (s *Store) Exercise(id int64) (Exercise, error) {
	return scanExercise(s.DB.QueryRow(`SELECT `+exerciseFields+` FROM exercises e JOIN labs l ON l.id=e.lab_id WHERE e.id=? AND l.archived=0 AND e.archived=0`, id))
}
func (s *Store) Exercises(uid int64) ([]Exercise, error) {
	rows, err := s.DB.Query(`SELECT ` + exerciseFields + ` FROM exercises e JOIN labs l ON l.id=e.lab_id WHERE l.archived=0 AND e.archived=0 ORDER BY l.position,e.position,e.id`)
	if err != nil {
		return nil, err
	}
	es := []Exercise{}
	for rows.Next() {
		e, er := scanExercise(rows)
		if er != nil {
			rows.Close()
			return nil, er
		}
		es = append(es, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range es {
		e := &es[i]
		var passed, clean, started int
		s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM submissions WHERE user_id=? AND exercise_id=? AND mode='check' AND status IN ('passed','warnings')),EXISTS(SELECT 1 FROM submissions WHERE user_id=? AND exercise_id=? AND mode='check' AND status='passed'),EXISTS(SELECT 1 FROM submissions WHERE user_id=? AND exercise_id=?) OR EXISTS(SELECT 1 FROM drafts WHERE user_id=? AND exercise_id=?)`, uid, e.ID, uid, e.ID, uid, e.ID, uid, e.ID).Scan(&passed, &clean, &started)
		e.Status = "not_started"
		if started != 0 {
			e.Status = "in_progress"
		}
		if passed != 0 {
			e.Status = "warnings"
		}
		if clean != 0 {
			e.Status = "passed"
		}
		if passed == 0 {
			var latest string
			s.DB.QueryRow(`SELECT status FROM submissions WHERE user_id=? AND exercise_id=? AND mode='check' ORDER BY id DESC LIMIT 1`, uid, e.ID).Scan(&latest)
			if latest == "review" || latest == "revision" {
				e.Status = latest
			}
		}
	}
	return es, nil
}
func (s *Store) Submissions(uid, exercise int64, all bool) ([]Submission, error) {
	query := `SELECT s.id,s.user_id,s.exercise_id,s.code,s.mode,s.status,s.report,s.created_at,e.title,u.full_name FROM submissions s JOIN exercises e ON e.id=s.exercise_id JOIN users u ON u.id=s.user_id WHERE 1=1`
	args := []any{}
	if !all {
		query += ` AND s.user_id=?`
		args = append(args, uid)
	}
	if exercise > 0 {
		query += ` AND s.exercise_id=?`
		args = append(args, exercise)
	}
	query += ` ORDER BY s.id DESC`
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Submission{}
	for rows.Next() {
		var v Submission
		var report string
		if err = rows.Scan(&v.ID, &v.UserID, &v.ExerciseID, &v.Code, &v.Mode, &v.Status, &report, &v.Created, &v.Title, &v.Login); err != nil {
			return nil, err
		}
		v.Report = json.RawMessage(report)
		out = append(out, v)
	}
	return out, rows.Err()
}
func Now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
