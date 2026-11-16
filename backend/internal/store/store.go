package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	rows, err := s.DB.Query(`SELECT id,title,description,position,source FROM labs ORDER BY position,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Lab{}
	for rows.Next() {
		var l Lab
		if err = rows.Scan(&l.ID, &l.Title, &l.Description, &l.Position, &l.Source); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

const exerciseFields = `e.id,e.lab_id,e.title,e.statement,e.template,e.tests,e.timeout_ms,e.memory_mb,e.difficulty,e.position`

func scanExercise(row interface{ Scan(...any) error }) (e Exercise, err error) {
	err = row.Scan(&e.ID, &e.LabID, &e.Title, &e.Statement, &e.Template, &e.Tests, &e.TimeoutMS, &e.MemoryMB, &e.Difficulty, &e.Position)
	e.TestCount = len(strings.Split(strings.TrimSpace(e.Tests), "\n"))
	return
}
func (s *Store) Exercise(id int64) (Exercise, error) {
	return scanExercise(s.DB.QueryRow(`SELECT `+exerciseFields+` FROM exercises e WHERE e.id=?`, id))
}
func (s *Store) Exercises(uid int64) ([]Exercise, error) {
	rows, err := s.DB.Query(`SELECT ` + exerciseFields + ` FROM exercises e JOIN labs l ON l.id=e.lab_id ORDER BY l.position,e.position,e.id`)
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
func (s *Store) Seed() error {
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM labs`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	type task struct{ title, statement, template, tests string }
	groups := []struct {
		title, description string
		tasks              []task
	}{
		{"Основы Python", "Ввод и вывод, переменные и арифметика", []task{
			{"Привет, Python!", "Прочитайте имя из стандартного ввода и выведите приветствие в формате «Привет, Анна!».\n\nВходные данные\nОдна строка — имя пользователя.\n\nВыходные данные\nПриветствие с именем и восклицательным знаком.", "name = input()\n# Выведите приветствие\n", `{"stdin":"Анна\n","stdout":"Привет, Анна!\n"}
{"stdin":"Python\n","stdout":"Привет, Python!\n"}
{"stdin":"Мир\n","stdout":"Привет, Мир!\n"}`},
			{"Сумма двух чисел", "Прочитайте два целых числа (каждое на отдельной строке) и выведите их сумму.", "a = int(input())\nb = int(input())\n# Выведите сумму\n", `{"stdin":"2\n3\n","stdout":"5\n"}
{"stdin":"-5\n5\n","stdout":"0\n"}
{"stdin":"100\n250\n","stdout":"350\n"}`},
			{"Минуты в часы", "На вход подаётся неотрицательное число минут. Выведите полные часы и оставшиеся минуты через пробел.", "minutes = int(input())\n", `{"stdin":"135\n","stdout":"2 15\n"}
{"stdin":"60\n","stdout":"1 0\n"}
{"stdin":"0\n","stdout":"0 0\n"}`},
		}},
		{"Условия и циклы", "Логика программы и повторение действий", []task{
			{"Чётное или нечётное", "Прочитайте целое число. Выведите EVEN для чётного числа и ODD для нечётного.", "number = int(input())\n", `{"stdin":"4\n","stdout":"EVEN\n"}
{"stdin":"-3\n","stdout":"ODD\n"}
{"stdin":"0\n","stdout":"EVEN\n"}`},
			{"Сумма последовательности", "Прочитайте целое n (1 ≤ n ≤ 10000) и выведите сумму целых чисел от 1 до n включительно.", "n = int(input())\n", `{"stdin":"5\n","stdout":"15\n"}
{"stdin":"1\n","stdout":"1\n"}
{"stdin":"100\n","stdout":"5050\n"}`},
			{"Факториал", "Прочитайте n (0 ≤ n ≤ 20) и выведите n!. Факториал нуля равен 1.", "n = int(input())\n", `{"stdin":"0\n","stdout":"1\n"}
{"stdin":"5\n","stdout":"120\n"}
{"stdin":"10\n","stdout":"3628800\n"}`},
		}},
		{"Строки и коллекции", "Работа с текстом, списками и множествами", []task{
			{"Палиндром", "Прочитайте строку из строчных латинских букв. Выведите YES, если она одинаково читается в обоих направлениях, иначе NO.", "text = input()\n", `{"stdin":"level\n","stdout":"YES\n"}
{"stdin":"python\n","stdout":"NO\n"}`},
			{"Уникальные числа", "Прочитайте целые числа в одной строке через пробел. Выведите количество различных чисел.", "numbers = list(map(int, input().split()))\n", `{"stdin":"1 2 2 3 1\n","stdout":"3\n"}
{"stdin":"5 5 5\n","stdout":"1\n"}`},
			{"Самое длинное слово", "Прочитайте строку слов через пробел. Выведите первое слово максимальной длины.", "words = input().split()\n", `{"stdin":"я изучаю Python\n","stdout":"изучаю\n"}
{"stdin":"one three two\n","stdout":"three\n"}`},
		}},
		{"Функции и алгоритмы", "Декомпозиция и простые алгоритмы", []task{
			{"Наибольший общий делитель", "Прочитайте два положительных целых числа на отдельных строках. Выведите их НОД.", "a = int(input())\nb = int(input())\n", `{"stdin":"18\n24\n","stdout":"6\n"}
{"stdin":"7\n13\n","stdout":"1\n"}`},
			{"Простое число", "Прочитайте n (1 ≤ n ≤ 1000000). Выведите YES, если число простое, иначе NO.", "n = int(input())\n", `{"stdin":"1\n","stdout":"NO\n"}
{"stdin":"17\n","stdout":"YES\n"}
{"stdin":"100\n","stdout":"NO\n"}`},
			{"Числа Фибоначчи", "Прочитайте n (0 ≤ n ≤ 40). Выведите n-е число Фибоначчи: F(0)=0, F(1)=1.", "n = int(input())\n", `{"stdin":"0\n","stdout":"0\n"}
{"stdin":"10\n","stdout":"55\n"}
{"stdin":"1\n","stdout":"1\n"}`},
		}},
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, g := range groups {
		res, er := tx.Exec(`INSERT INTO labs(title,description,position,source) VALUES(?,?,?,'demo')`, g.title, g.description, i+1)
		if er != nil {
			return er
		}
		id, _ := res.LastInsertId()
		for j, t := range g.tasks {
			_, er = tx.Exec(`INSERT INTO exercises(lab_id,title,statement,template,tests,position) VALUES(?,?,?,?,?,?)`, id, t.title, t.statement, t.template, t.tests, j+1)
			if er != nil {
				return fmt.Errorf("seed: %w", er)
			}
		}
	}
	return tx.Commit()
}
func Now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
