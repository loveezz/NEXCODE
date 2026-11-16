package httpapi

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"nexcode/internal/auth"
	"nexcode/internal/engine"
	"nexcode/internal/store"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type userKey struct{}
type bucket struct {
	Count int
	Until time.Time
}
type API struct {
	Store          *store.Store
	Auth           *auth.Service
	Queue          *engine.Queue
	AppURL         string
	mu             sync.Mutex
	submitMu       sync.Mutex
	limits         map[string]bucket
	trustedProxies []*net.IPNet
}

func New(s *store.Store, a *auth.Service, q *engine.Queue, url string) http.Handler {
	api := &API{Store: s, Auth: a, Queue: q, AppURL: strings.TrimRight(url, "/"), limits: map[string]bucket{}}
	for _, value := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		if strings.TrimSpace(value) == "" {
			continue
		}
		_, network, err := net.ParseCIDR(strings.TrimSpace(value))
		if err != nil {
			log.Fatal("Invalid TRUSTED_PROXY_CIDRS")
		}
		api.trustedProxies = append(api.trustedProxies, network)
	}
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]any{"status": "ok"}) })
	m.HandleFunc("GET /api/auth/options", api.loginOptions)
	m.HandleFunc("POST /api/auth/login", api.studentLogin)
	m.HandleFunc("POST /api/auth/staff/login", api.staffLogin)
	m.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		a.Logout(w, r)
		write(w, 200, map[string]bool{"ok": true})
	})

	m.HandleFunc("GET /api/auth/me", api.guard("student", func(w http.ResponseWriter, r *http.Request) { write(w, 200, user(r)) }))
	m.HandleFunc("GET /api/catalog", api.guard("student", api.catalog))
	m.HandleFunc("GET /api/exercises/{id}", api.guard("student", api.exercise))
	m.HandleFunc("GET /api/exercises/{id}/draft", api.guard("student", api.getDraft))
	m.HandleFunc("PUT /api/exercises/{id}/draft", api.guard("student", api.putDraft))
	m.HandleFunc("POST /api/exercises/{id}/submit", api.guard("student", api.submit))
	m.HandleFunc("GET /api/submissions", api.guard("student", api.submissions))
	m.HandleFunc("GET /api/submissions/{id}", api.guard("student", api.submission))
	m.HandleFunc("GET /api/teacher/students", api.guard("teacher", api.students))
	m.HandleFunc("GET /api/teacher/export", api.guard("teacher", api.export))
	m.HandleFunc("PATCH /api/teacher/submissions/{id}/review", api.guard("teacher", api.reviewSubmission))
	m.HandleFunc("GET /api/teacher/catalog", api.guard("teacher", api.fullCatalog))
	m.HandleFunc("POST /api/teacher/labs", api.guard("teacher", api.saveLab))
	m.HandleFunc("PUT /api/teacher/labs/{id}", api.guard("teacher", api.saveLab))
	m.HandleFunc("DELETE /api/teacher/labs/{id}", api.guard("teacher", api.deleteLab))
	m.HandleFunc("POST /api/teacher/exercises", api.guard("teacher", api.saveExercise))
	m.HandleFunc("PUT /api/teacher/exercises/{id}", api.guard("teacher", api.saveExercise))
	m.HandleFunc("DELETE /api/teacher/exercises/{id}", api.guard("teacher", api.deleteExercise))
	m.HandleFunc("POST /api/teacher/import", api.guard("teacher", api.importCatalog))
	m.HandleFunc("GET /api/admin/users", api.guard("admin", api.users))
	m.HandleFunc("PATCH /api/admin/users/{id}", api.guard("admin", api.updateUser))
	m.HandleFunc("GET /api/teacher/identity", api.guard("teacher", api.identitySettings))
	m.HandleFunc("PUT /api/teacher/identity", api.guard("teacher", api.saveIdentitySettings))
	m.HandleFunc("POST /api/teacher/groups", api.guard("teacher", api.saveGroup))
	m.HandleFunc("PUT /api/teacher/groups/{id}", api.guard("teacher", api.saveGroup))
	m.HandleFunc("GET /api/teacher/groups/{id}/invites", api.guard("teacher", api.invites))
	m.HandleFunc("POST /api/teacher/groups/{id}/invites", api.guard("teacher", api.importInvites))
	m.HandleFunc("DELETE /api/teacher/groups/{id}/sessions", api.guard("teacher", api.clearGroupSessions))
	m.HandleFunc("DELETE /api/teacher/students/{id}/sessions", api.guard("teacher", api.clearStudentSessions))
	m.HandleFunc("GET /api/teacher/login-events", api.guard("teacher", api.loginEvents))
	m.HandleFunc("GET /api/admin/metrics", api.guard("admin", api.metrics))
	return api.middleware(m)
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	write(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "Некорректный JSON или превышен размер запроса")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		fail(w, 400, "Ожидался один JSON-объект")
		return false
	}
	return true
}

func user(r *http.Request) store.User { return r.Context().Value(userKey{}).(store.User) }
func id(r *http.Request) int64        { v, _ := strconv.ParseInt(r.PathValue("id"), 10, 64); return v }
func roleLevel(v string) int {
	switch v {
	case "admin":
		return 3
	case "teacher":
		return 2
	case "student":
		return 1
	}
	return 0
}
func (a *API) guard(role string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, sessionID, err := a.Auth.Read(r)
		if err != nil {
			fail(w, 401, "Войдите в аккаунт")
			return
		}
		if roleLevel(u.Role) < roleLevel(role) {
			fail(w, 403, "Недостаточно прав")
			return
		}
		a.Auth.Refresh(w, sessionID)
		h(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	}
}
func (a *API) allow(key string, max int, period time.Duration) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if len(a.limits) > 10000 {
		for k, v := range a.limits {
			if now.After(v.Until) {
				delete(a.limits, k)
			}
		}
	}
	b := a.limits[key]
	if now.After(b.Until) {
		b = bucket{Until: now.Add(period)}
	}
	b.Count++
	a.limits[key] = b
	return b.Count <= max
}
func (a *API) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	for _, network := range a.trustedProxies {
		if peer != nil && network.Contains(peer) {
			forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
			if address := net.ParseIP(forwarded); address != nil {
				return address.String()
			}
			break
		}
	}
	return host
}
func (a *API) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic: %v", v)
				fail(w, 500, "Внутренняя ошибка сервера")
			}
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
		}()
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != a.AppURL {
				fail(w, 403, "Недопустимый источник запроса")
				return
			}
		}
		if !a.allow("global:"+a.clientIP(r), 6000, time.Minute) {
			w.Header().Set("Retry-After", "60")
			fail(w, 429, "Слишком много запросов")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *API) catalog(w http.ResponseWriter, r *http.Request) {
	labs, err := a.Store.Labs()
	if err != nil {
		fail(w, 500, "Ошибка чтения каталога")
		return
	}
	es, err := a.Store.Exercises(user(r).ID)
	if err != nil {
		fail(w, 500, "Ошибка чтения упражнений")
		return
	}
	for i := range es {
		es[i].Tests = ""
	}
	write(w, 200, map[string]any{"labs": labs, "exercises": es})
}
func (a *API) fullCatalog(w http.ResponseWriter, r *http.Request) {
	labs, _ := a.Store.Labs()
	es, err := a.Store.Exercises(user(r).ID)
	if err != nil {
		fail(w, 500, "Ошибка чтения каталога")
		return
	}
	write(w, 200, map[string]any{"labs": labs, "exercises": es})
}
func (a *API) exercise(w http.ResponseWriter, r *http.Request) {
	e, err := a.Store.Exercise(id(r))
	if err != nil {
		fail(w, 404, "Упражнение не найдено")
		return
	}
	tests, _ := engine.ParseTests(e.Tests)
	e.Tests = ""
	example := engine.Test{}
	if len(tests) > 0 {
		example = tests[0]
	}
	write(w, 200, map[string]any{"exercise": e, "example": example})
}
func (a *API) getDraft(w http.ResponseWriter, r *http.Request) {
	var code, updated string
	err := a.Store.DB.QueryRow(`SELECT code,updated_at FROM drafts WHERE user_id=? AND exercise_id=?`, user(r).ID, id(r)).Scan(&code, &updated)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		fail(w, 500, "Не удалось загрузить черновик")
		return
	}
	write(w, 200, map[string]any{"code": code, "exists": err == nil, "updated_at": updated})
}
func (a *API) putDraft(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &v) {
		return
	}
	if len(v.Code) > 65536 {
		fail(w, 400, "Код превышает 64 КБ")
		return
	}
	if _, err := a.Store.Exercise(id(r)); err != nil {
		fail(w, 404, "Упражнение не найдено")
		return
	}
	now := store.Now()
	_, err := a.Store.DB.Exec(`INSERT INTO drafts(user_id,exercise_id,code,updated_at) VALUES(?,?,?,?) ON CONFLICT(user_id,exercise_id) DO UPDATE SET code=excluded.code,updated_at=excluded.updated_at`, user(r).ID, id(r), v.Code, now)
	if err != nil {
		fail(w, 500, "Не удалось сохранить черновик")
		return
	}
	write(w, 200, map[string]string{"updated_at": now})
}
func (a *API) submit(w http.ResponseWriter, r *http.Request) {
	a.submitMu.Lock()
	defer a.submitMu.Unlock()
	uid := user(r).ID
	if !a.allow(fmt.Sprintf("run:%d", uid), 15, time.Minute) {
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "Лимит: 15 запусков в минуту")
		return
	}
	var v struct {
		Code  string `json:"code"`
		Mode  string `json:"mode"`
		Stdin string `json:"stdin"`
	}
	if !decode(w, r, &v) {
		return
	}
	if len(v.Code) == 0 || len(v.Code) > 65536 || len(v.Stdin) > 32768 || (v.Mode != "run" && v.Mode != "check") {
		fail(w, 400, "Укажите код до 64 КБ и режим run или check")
		return
	}
	e, err := a.Store.Exercise(id(r))
	if err != nil {
		fail(w, 404, "Упражнение не найдено")
		return
	}
	if e.Kind == "written" && v.Mode == "run" {
		fail(w, 400, "Письменный ответ нужно сдать преподавателю")
		return
	}
	if e.Kind != "python" && v.Mode == "check" {
		report := `{"review":"pending","feedback":"Работа отправлена преподавателю. Автоматическая оценка не выставляется."}`
		res, err := a.Store.DB.Exec(`INSERT INTO submissions(user_id,exercise_id,code,mode,status,report) VALUES(?,?,?,'check','review',?)`, uid, e.ID, v.Code, report)
		if err != nil {
			fail(w, 500, "Не удалось сохранить работу")
			return
		}
		sid, _ := res.LastInsertId()
		write(w, 202, map[string]any{"id": sid, "status": "review"})
		return
	}
	var pending int
	a.Store.DB.QueryRow(`SELECT COUNT(*) FROM submissions WHERE user_id=? AND status IN ('queued','running')`, uid).Scan(&pending)
	if pending >= 2 {
		fail(w, 429, "Дождитесь завершения предыдущих запусков")
		return
	}
	res, err := a.Store.DB.Exec(`INSERT INTO submissions(user_id,exercise_id,code,mode,status) VALUES(?,?,?,?,'queued')`, uid, e.ID, v.Code, v.Mode)
	if err != nil {
		fail(w, 500, "Ошибка сохранения отправки")
		return
	}
	sid, _ := res.LastInsertId()
	if !a.Queue.Submit(engine.Job{ID: sid, Exercise: e, Code: v.Code, Mode: v.Mode, Stdin: v.Stdin}) {
		a.Store.DB.Exec(`UPDATE submissions SET status='error',report='{"error":"Очередь заполнена"}' WHERE id=?`, sid)
		fail(w, 503, "Очередь заполнена. Повторите позже")
		return
	}
	write(w, 202, map[string]any{"id": sid, "status": "queued"})
}
func (a *API) submissions(w http.ResponseWriter, r *http.Request) {
	exercise, _ := strconv.ParseInt(r.URL.Query().Get("exercise_id"), 10, 64)
	u := user(r)
	all := roleLevel(u.Role) >= 2 && r.URL.Query().Get("all") == "1"
	out, err := a.Store.Submissions(u.ID, exercise, all)
	if err != nil {
		fail(w, 500, "Ошибка чтения истории")
		return
	}
	write(w, 200, out)
}
func (a *API) submission(w http.ResponseWriter, r *http.Request) {
	var s store.Submission
	var report string
	err := a.Store.DB.QueryRow(`SELECT s.id,s.user_id,s.exercise_id,s.code,s.mode,s.status,s.report,s.created_at,e.title FROM submissions s JOIN exercises e ON e.id=s.exercise_id WHERE s.id=?`, id(r)).Scan(&s.ID, &s.UserID, &s.ExerciseID, &s.Code, &s.Mode, &s.Status, &report, &s.Created, &s.Title)
	if err != nil {
		fail(w, 404, "Отправка не найдена")
		return
	}
	if s.UserID != user(r).ID && roleLevel(user(r).Role) < 2 {
		fail(w, 403, "Недостаточно прав")
		return
	}
	s.Report = json.RawMessage(report)
	write(w, 200, s)
}
func (a *API) users(w http.ResponseWriter, r *http.Request) {
	out, err := a.userList(false)
	if err != nil {
		fail(w, 500, "Ошибка чтения пользователей")
		return
	}
	write(w, 200, out)
}
func (a *API) userList(studentsOnly bool) ([]store.User, error) { return a.Store.Users(studentsOnly) }
func (a *API) students(w http.ResponseWriter, r *http.Request) {
	us, err := a.userList(true)
	if err != nil {
		fail(w, 500, "Ошибка чтения студентов")
		return
	}
	out := []any{}
	for _, u := range us {
		es, er := a.Store.Exercises(u.ID)
		if er != nil {
			fail(w, 500, "Ошибка чтения прогресса")
			return
		}
		progress := []map[string]any{}
		solved := 0
		for _, e := range es {
			progress = append(progress, map[string]any{"exercise_id": e.ID, "title": e.Title, "status": e.Status})
			if e.Status == "passed" || e.Status == "warnings" {
				solved++
			}
		}
		out = append(out, map[string]any{"user": u, "solved": solved, "total": len(es), "progress": progress})
	}
	write(w, 200, out)
}
func csvSafe(s string) string {
	if strings.HasPrefix(s, "=") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "@") || strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}
func (a *API) export(w http.ResponseWriter, r *http.Request) {
	us, err := a.userList(true)
	if err != nil {
		fail(w, 500, "Ошибка экспорта")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=nexcode-progress.csv")
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	c := csv.NewWriter(w)
	c.Write([]string{"student_id", "full_name", "group_name", "lab_id", "exercise_id", "exercise", "status"})
	for _, u := range us {
		es, _ := a.Store.Exercises(u.ID)
		for _, e := range es {
			c.Write([]string{fmt.Sprint(u.ID), csvSafe(u.FullName), csvSafe(u.GroupName), fmt.Sprint(e.LabID), fmt.Sprint(e.ID), csvSafe(e.Title), e.Status})
		}
	}
	c.Flush()
}
func validateLab(l store.Lab) error {
	if len(strings.TrimSpace(l.Title)) < 2 || len(l.Title) > 200 || len(l.Description) > 10000 {
		return errors.New("Проверьте название и описание лабораторной")
	}
	return nil
}
func validateExercise(e *store.Exercise) error {
	if len(strings.TrimSpace(e.Title)) < 2 || len(e.Title) > 200 || len(e.Statement) == 0 || len(e.Statement) > 50000 || len(e.Template) > 65536 {
		return errors.New("Проверьте название, условие и шаблон")
	}
	if e.TimeoutMS == 0 {
		e.TimeoutMS = 5000
	}
	if e.MemoryMB == 0 {
		e.MemoryMB = 128
	}
	if e.TimeoutMS < 100 || e.TimeoutMS > 5000 || e.MemoryMB < 64 || e.MemoryMB > 256 {
		return errors.New("Время: 100–5000 мс, память: 64–256 МБ")
	}
	if e.Difficulty == "" {
		e.Difficulty = "Базовый"
	}
	if e.Kind == "" {
		e.Kind = "python"
	}
	if e.Kind != "python" && e.Kind != "written" && e.Kind != "python_review" {
		return errors.New("Неизвестный формат задания")
	}
	if e.Kind == "written" {
		return nil
	}
	_, err := engine.ParseTests(e.Tests)
	return err
}
func (a *API) saveLab(w http.ResponseWriter, r *http.Request) {
	var l store.Lab
	if !decode(w, r, &l) {
		return
	}
	if err := validateLab(l); err != nil {
		fail(w, 400, err.Error())
		return
	}
	var err error
	lid := id(r)
	if lid > 0 {
		var res sql.Result
		res, err = a.Store.DB.Exec(`UPDATE labs SET title=?,description=?,position=?,source='custom' WHERE id=?`, l.Title, l.Description, l.Position, lid)
		if err == nil {
			n, _ := res.RowsAffected()
			if n == 0 {
				fail(w, 404, "Лабораторная не найдена")
				return
			}
		}
	} else {
		var res sql.Result
		res, err = a.Store.DB.Exec(`INSERT INTO labs(title,description,position,source) VALUES(?,?,?,'custom')`, l.Title, l.Description, l.Position)
		if err == nil {
			lid, _ = res.LastInsertId()
		}
	}
	if err != nil {
		fail(w, 500, "Ошибка сохранения лабораторной")
		return
	}
	write(w, 200, map[string]int64{"id": lid})
}
func (a *API) saveExercise(w http.ResponseWriter, r *http.Request) {
	var e store.Exercise
	if !decode(w, r, &e) {
		return
	}
	if err := validateExercise(&e); err != nil {
		fail(w, 400, err.Error())
		return
	}
	eid := id(r)
	var res sql.Result
	var err error
	if eid > 0 {
		res, err = a.Store.DB.Exec(`UPDATE exercises SET lab_id=?,title=?,statement=?,template=?,tests=?,timeout_ms=?,memory_mb=?,difficulty=?,position=?,kind=? WHERE id=?`, e.LabID, e.Title, e.Statement, e.Template, e.Tests, e.TimeoutMS, e.MemoryMB, e.Difficulty, e.Position, e.Kind, eid)
	} else {
		res, err = a.Store.DB.Exec(`INSERT INTO exercises(lab_id,title,statement,template,tests,timeout_ms,memory_mb,difficulty,position,kind) VALUES(?,?,?,?,?,?,?,?,?,?)`, e.LabID, e.Title, e.Statement, e.Template, e.Tests, e.TimeoutMS, e.MemoryMB, e.Difficulty, e.Position, e.Kind)
	}
	if err != nil {
		fail(w, 400, "Ошибка сохранения. Проверьте лабораторную")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "Упражнение не найдено")
		return
	}
	if eid == 0 {
		eid, _ = res.LastInsertId()
	}
	write(w, 200, map[string]int64{"id": eid})
}
func (a *API) deleteExercise(w http.ResponseWriter, r *http.Request) {
	tx, err := a.Store.DB.Begin()
	if err != nil {
		fail(w, 500, "Ошибка удаления")
		return
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM drafts WHERE exercise_id=?`, id(r))
	if _, err = tx.Exec(`DELETE FROM exercises WHERE id=?`, id(r)); err != nil {
		fail(w, 409, "Упражнение имеет отправки: история должна быть сохранена")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "Ошибка удаления")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *API) deleteLab(w http.ResponseWriter, r *http.Request) {
	if _, err := a.Store.DB.Exec(`DELETE FROM labs WHERE id=?`, id(r)); err != nil {
		fail(w, 409, "Сначала удалите упражнения лабораторной")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

type Import struct {
	Labs []struct {
		store.Lab
		Exercises []store.Exercise `json:"exercises"`
	} `json:"labs"`
}

func (a *API) importCatalog(w http.ResponseWriter, r *http.Request) {
	var data Import
	if !decode(w, r, &data) {
		return
	}
	if len(data.Labs) == 0 || len(data.Labs) > 50 {
		fail(w, 400, "Импорт должен содержать от 1 до 50 лабораторных")
		return
	}
	for _, l := range data.Labs {
		if err := validateLab(l.Lab); err != nil {
			fail(w, 400, err.Error())
			return
		}
		if len(l.Exercises) > 100 {
			fail(w, 400, "Не более 100 упражнений на лабораторную")
			return
		}
		for _, e := range l.Exercises {
			if err := validateExercise(&e); err != nil {
				fail(w, 400, e.Title+": "+err.Error())
				return
			}
		}
	}
	tx, err := a.Store.DB.Begin()
	if err != nil {
		fail(w, 500, "Ошибка импорта")
		return
	}
	defer tx.Rollback()
	count := 0
	for _, l := range data.Labs {
		res, er := tx.Exec(`INSERT INTO labs(title,description,position,source) VALUES(?,?,?,'custom')`, l.Title, l.Description, l.Position)
		if er != nil {
			fail(w, 500, "Ошибка импорта лабораторной")
			return
		}
		lid, _ := res.LastInsertId()
		for _, e := range l.Exercises {
			validateExercise(&e)
			_, er = tx.Exec(`INSERT INTO exercises(lab_id,title,statement,template,tests,timeout_ms,memory_mb,difficulty,position,kind) VALUES(?,?,?,?,?,?,?,?,?,?)`, lid, e.Title, e.Statement, e.Template, e.Tests, e.TimeoutMS, e.MemoryMB, e.Difficulty, e.Position, e.Kind)
			if er != nil {
				fail(w, 500, "Ошибка импорта упражнения")
				return
			}
			count++
		}
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "Ошибка импорта")
		return
	}
	write(w, 201, map[string]int{"labs": len(data.Labs), "exercises": count})
}
func (a *API) updateUser(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Role    string `json:"role"`
		Blocked bool   `json:"blocked"`
	}
	if !decode(w, r, &v) {
		return
	}
	if roleLevel(v.Role) == 0 {
		fail(w, 400, "Неизвестная роль")
		return
	}
	if id(r) == user(r).ID && (v.Role != "admin" || v.Blocked) {
		fail(w, 400, "Нельзя понизить или заблокировать собственный аккаунт")
		return
	}
	tx, err := a.Store.DB.Begin()
	if err != nil {
		fail(w, 500, "Ошибка изменения пользователя")
		return
	}
	defer tx.Rollback()
	var role string
	err = tx.QueryRow(`SELECT role FROM users WHERE id=?`, id(r)).Scan(&role)
	if err != nil {
		fail(w, 404, "Пользователь не найден")
		return
	}
	if (role == "student") != (v.Role == "student") {
		fail(w, 400, "Служебные аккаунты создаются через конфигурацию. Ученический вход не может получить служебную роль")
		return
	}
	if role == "admin" && (v.Role != "admin" || v.Blocked) {
		var n int
		tx.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin' AND blocked=0 AND id<>?`, id(r)).Scan(&n)
		if n == 0 {
			fail(w, 400, "Нужен хотя бы один активный администратор")
			return
		}
	}
	if _, err = tx.Exec(`UPDATE users SET role=?,blocked=? WHERE id=?`, v.Role, v.Blocked, id(r)); err != nil {
		fail(w, 500, "Ошибка изменения пользователя")
		return
	}
	tx.Exec(`DELETE FROM sessions WHERE user_id=? AND ?=1`, id(r), v.Blocked)
	if err = tx.Commit(); err != nil {
		fail(w, 500, "Ошибка изменения пользователя")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	var sessions, users, submissions int
	a.Store.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE expires>?`, time.Now().Unix()).Scan(&sessions)
	a.Store.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	a.Store.DB.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&submissions)
	write(w, 200, map[string]any{"queued": a.Queue.Pending(), "running": a.Queue.Active.Load(), "active_sessions": sessions, "users": users, "submissions": submissions})
}
