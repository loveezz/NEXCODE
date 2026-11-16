package httpapi

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"nexcode/internal/auth"
	"nexcode/internal/store"
	"strconv"
	"strings"
	"time"
)

func (a *API) loginOptions(w http.ResponseWriter, r *http.Request) {
	groups, err := a.Store.Groups()
	if err != nil {
		fail(w, 500, "Ошибка чтения групп")
		return
	}
	var mode string
	if err = a.Store.DB.QueryRow(`SELECT value FROM settings WHERE key='group_mode'`).Scan(&mode); err != nil {
		fail(w, 500, "Ошибка настроек входа")
		return
	}
	date, err := a.Auth.SemesterEnd()
	if err != nil {
		fail(w, 500, "Ошибка срока сессий")
		return
	}
	public := []map[string]any{}
	for _, g := range groups {
		public = append(public, map[string]any{"id": g.ID, "name": g.Name, "invite_required": g.InviteRequired})
	}
	write(w, 200, map[string]any{"groups": public, "group_mode": mode, "semester_end": date})
}
func (a *API) studentLogin(w http.ResponseWriter, r *http.Request) {
	var v struct {
		FullName  string `json:"full_name"`
		GroupName string `json:"group_name"`
		Code      string `json:"code"`
	}
	clientIP := a.clientIP(r)
	if !a.allow("identity-login:"+clientIP, 10, time.Minute) {
		a.Auth.Denied("", "", clientIP, "rate_limit", "student")
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "Слишком много попыток входа. Повторите через минуту")
		return
	}
	if !decode(w, r, &v) {
		a.Auth.Denied("", "", clientIP, "invalid_json", "student")
		return
	}
	name, errName := store.NormalizeIdentity(v.FullName, 150)
	group, errGroup := store.NormalizeIdentity(v.GroupName, 20)
	if errName != nil || errGroup != nil || len(v.Code) > 64 {
		a.Auth.Denied(shortText(v.FullName, 150), shortText(v.GroupName, 20), clientIP, "validation", "student")
		fail(w, 400, "Укажите ФИО (до 150 символов) и группу (до 20 символов)")
		return
	}
	u, err := a.Auth.StudentLogin(w, name, group, v.Code, clientIP)
	if err != nil {
		a.Auth.Denied(name, group, clientIP, "denied", "student")
		if errors.Is(err, auth.ErrDenied) {
			fail(w, 401, auth.ErrDenied.Error())
		} else {
			fail(w, 503, "Вход временно недоступен. Повторите попытку")
		}
		return
	}
	write(w, 200, u)
}
func shortText(value string, n int) string {
	runes := []rune(strings.Join(strings.Fields(value), " "))
	if len(runes) > n {
		runes = runes[:n]
	}
	return string(runes)
}
func (a *API) staffLogin(w http.ResponseWriter, r *http.Request) {
	clientIP := a.clientIP(r)
	if !a.allow("identity-login:"+clientIP, 10, time.Minute) {
		a.Auth.Denied("", "", clientIP, "rate_limit", "staff")
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "Слишком много попыток входа. Повторите через минуту")
		return
	}
	var v struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !decode(w, r, &v) {
		a.Auth.Denied("", "", clientIP, "invalid_json", "staff")
		return
	}
	if len(v.Login) > 100 || len(v.Password) > 72 {
		a.Auth.Denied(shortText(v.Login, 100), "", clientIP, "validation", "staff")
		fail(w, 401, "Не удалось войти. Проверьте логин и пароль.")
		return
	}
	u, err := a.Auth.StaffLogin(w, v.Login, v.Password, clientIP)
	if err != nil {
		a.Auth.Denied(shortText(v.Login, 100), "", clientIP, "denied", "staff")
		if errors.Is(err, auth.ErrDenied) {
			fail(w, 401, "Не удалось войти. Проверьте логин и пароль.")
		} else {
			fail(w, 503, "Вход временно недоступен")
		}
		return
	}
	write(w, 200, u)
}
func (a *API) identitySettings(w http.ResponseWriter, r *http.Request) {
	groups, err := a.Store.Groups()
	if err != nil {
		fail(w, 500, "Ошибка чтения групп")
		return
	}
	var mode string
	a.Store.DB.QueryRow(`SELECT value FROM settings WHERE key='group_mode'`).Scan(&mode)
	date, _ := a.Auth.SemesterEnd()
	write(w, 200, map[string]any{"groups": groups, "group_mode": mode, "semester_end": date})
}
func (a *API) saveIdentitySettings(w http.ResponseWriter, r *http.Request) {
	var v struct {
		GroupMode   string `json:"group_mode"`
		SemesterEnd string `json:"semester_end"`
	}
	if !decode(w, r, &v) {
		return
	}
	if v.GroupMode != "free" && v.GroupMode != "list" {
		fail(w, 400, "Режим группы: free или list")
		return
	}
	expiry, err := a.Auth.SemesterExpiry(v.SemesterEnd)
	if err != nil || !expiry.After(time.Now()) || expiry.After(time.Now().AddDate(1, 0, 0)) {
		fail(w, 400, "Выберите дату от сегодняшнего дня до одного года вперёд")
		return
	}
	tx, err := a.Store.DB.Begin()
	if err != nil {
		fail(w, 500, "Ошибка сохранения настроек")
		return
	}
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE settings SET value=? WHERE key='group_mode'`, v.GroupMode)
	if err == nil {
		_, err = tx.Exec(`UPDATE settings SET value=? WHERE key='semester_end'`, v.SemesterEnd)
	}
	if err == nil {
		_, err = tx.Exec(`UPDATE sessions SET expires=? WHERE kind='student'`, expiry.Unix())
	}
	if err != nil {
		fail(w, 500, "Ошибка обновления сессий")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "Ошибка сохранения")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *API) saveGroup(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Name           string `json:"name"`
		InviteRequired bool   `json:"invite_required"`
	}
	if !decode(w, r, &v) {
		return
	}
	name, err := store.NormalizeIdentity(v.Name, 20)
	if err != nil {
		fail(w, 400, "Название группы: от 1 до 20 символов")
		return
	}
	tx, err := a.Store.DB.Begin()
	if err != nil {
		fail(w, 500, "Ошибка сохранения группы")
		return
	}
	defer tx.Rollback()
	gid := id(r)
	if gid == 0 {
		res, er := tx.Exec(`INSERT INTO groups(name,invite_required) VALUES(?,?)`, name, v.InviteRequired)
		if er != nil {
			fail(w, 409, "Группа уже существует")
			return
		}
		gid, _ = res.LastInsertId()
	} else {
		var previous string
		var required bool
		if err = tx.QueryRow(`SELECT name,invite_required FROM groups WHERE id=?`, gid).Scan(&previous, &required); err != nil {
			fail(w, 404, "Группа не найдена")
			return
		}
		if !v.InviteRequired {
			var duplicates int
			if err = tx.QueryRow(`SELECT COUNT(*) FROM (SELECT full_name FROM users WHERE role='student' AND group_name=? GROUP BY full_name HAVING COUNT(*)>1)`, previous).Scan(&duplicates); err != nil {
				fail(w, 500, "Ошибка чтения группы")
				return
			}
			if duplicates > 0 {
				fail(w, 409, "В группе есть полные тёзки. Для них нужно сохранить вход по кодам")
				return
			}
		}
		if _, err = tx.Exec(`UPDATE groups SET name=?,invite_required=? WHERE id=?`, name, v.InviteRequired, gid); err != nil {
			fail(w, 409, "Группа с таким именем уже существует")
			return
		}
		if _, err = tx.Exec(`UPDATE users SET group_name=? WHERE role='student' AND group_name=?`, name, previous); err != nil {
			fail(w, 409, "Не удалось переименовать группу")
			return
		}
		if required != v.InviteRequired {
			if _, err = tx.Exec(`DELETE FROM sessions WHERE kind='student' AND user_id IN(SELECT id FROM users WHERE group_name=?)`, name); err != nil {
				fail(w, 500, "Ошибка очистки сессий")
				return
			}
			tx.Exec(`UPDATE users SET session_uuid=NULL WHERE role='student' AND group_name=?`, name)
		}
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "Ошибка сохранения группы")
		return
	}
	write(w, 200, map[string]int64{"id": gid})
}

type invite struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	Code     string `json:"code"`
}

func (a *API) invites(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Store.DB.Query(`SELECT id,full_name,code FROM invite_codes WHERE group_id=? ORDER BY full_name,id`, id(r))
	if err != nil {
		fail(w, 500, "Ошибка чтения списка")
		return
	}
	defer rows.Close()
	out := []invite{}
	for rows.Next() {
		var v invite
		if err = rows.Scan(&v.ID, &v.FullName, &v.Code); err != nil {
			fail(w, 500, "Ошибка чтения списка")
			return
		}
		out = append(out, v)
	}
	write(w, 200, out)
}
func (a *API) importInvites(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Students []invite `json:"students"`
	}
	if !decode(w, r, &v) {
		return
	}
	if len(v.Students) == 0 || len(v.Students) > 200 {
		fail(w, 400, "В списке должно быть от 1 до 200 студентов")
		return
	}
	seen := map[string]bool{}
	for i := range v.Students {
		row := &v.Students[i]
		var err error
		row.FullName, err = store.NormalizeIdentity(row.FullName, 150)
		row.Code = strings.ToUpper(strings.TrimSpace(row.Code))
		if err != nil || !auth.ValidCode(row.Code) || seen[row.Code] {
			fail(w, 400, fmt.Sprintf("Строка %d: проверьте ФИО и уникальный код из 4–8 латинских букв или цифр", i+1))
			return
		}
		seen[row.Code] = true
	}
	tx, err := a.Store.DB.Begin()
	if err != nil {
		fail(w, 500, "Ошибка импорта")
		return
	}
	defer tx.Rollback()
	var group string
	if err = tx.QueryRow(`SELECT name FROM groups WHERE id=?`, id(r)).Scan(&group); err != nil {
		fail(w, 404, "Группа не найдена")
		return
	}
	for _, row := range v.Students {
		var iid int64
		var previous string
		err = tx.QueryRow(`SELECT id,full_name FROM invite_codes WHERE code=? AND group_id=?`, row.Code, id(r)).Scan(&iid, &previous)
		if err == nil {
			if previous != row.FullName {
				fail(w, 409, "Код уже привязан к другому студенту. Выдайте новый код")
				return
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			fail(w, 500, "Ошибка импорта")
			return
		}
		if _, err = tx.Exec(`INSERT INTO invite_codes(group_id,code,full_name) VALUES(?,?,?)`, id(r), row.Code, row.FullName); err != nil {
			fail(w, 409, "Один из кодов уже используется. Измените его")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "Ошибка импорта списка")
		return
	}
	write(w, 201, map[string]int{"imported": len(v.Students)})
}
func (a *API) clearGroupSessions(w http.ResponseWriter, r *http.Request) {
	var group string
	if err := a.Store.DB.QueryRow(`SELECT name FROM groups WHERE id=?`, id(r)).Scan(&group); err != nil {
		fail(w, 404, "Группа не найдена")
		return
	}
	a.clearSessions(w, `SELECT id FROM users WHERE role='student' AND group_name=?`, group)
}
func (a *API) clearStudentSessions(w http.ResponseWriter, r *http.Request) {
	u, err := a.Store.User(id(r))
	if err != nil || u.Role != "student" {
		fail(w, 404, "Студент не найден")
		return
	}
	a.clearSessions(w, `SELECT id FROM users WHERE id=? AND role='student'`, u.ID)
}
func (a *API) clearSessions(w http.ResponseWriter, query string, arg any) {
	tx, err := a.Store.DB.Begin()
	if err != nil {
		fail(w, 500, "Ошибка очистки сессий")
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM sessions WHERE kind='student' AND user_id IN(`+query+`)`, arg)
	if err != nil {
		fail(w, 500, "Ошибка очистки сессий")
		return
	}
	if _, err = tx.Exec(`UPDATE users SET session_uuid=NULL WHERE id IN(`+query+`)`, arg); err != nil {
		fail(w, 500, "Ошибка очистки сессий")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "Ошибка очистки сессий")
		return
	}
	n, _ := res.RowsAffected()
	write(w, 200, map[string]int64{"cleared": n})
}
func (a *API) loginEvents(w http.ResponseWriter, r *http.Request) {
	query := `SELECT id,user_id,full_name,group_name,ip,success,created_at FROM login_events WHERE kind='student'`
	args := []any{}
	params := r.URL.Query()
	if group := params.Get("group"); group != "" {
		normalized, err := store.NormalizeIdentity(group, 20)
		if err != nil {
			fail(w, 400, "Некорректная группа")
			return
		}
		query += ` AND group_name=?`
		args = append(args, normalized)
	}
	if student := params.Get("student_id"); student != "" {
		uid, err := strconv.ParseInt(student, 10, 64)
		if err != nil || uid < 1 {
			fail(w, 400, "Некорректный студент")
			return
		}
		u, er := a.Store.User(uid)
		if er != nil || u.Role != "student" {
			fail(w, 404, "Студент не найден")
			return
		}
		query += ` AND (user_id=? OR (user_id IS NULL AND full_name=? AND group_name=?))`
		args = append(args, uid, u.FullName, u.GroupName)
	}
	if before := params.Get("before"); before != "" {
		n, err := strconv.ParseInt(before, 10, 64)
		if err != nil || n < 1 {
			fail(w, 400, "Некорректная страница")
			return
		}
		query += ` AND id<?`
		args = append(args, n)
	}
	query += ` ORDER BY id DESC LIMIT 101`
	rows, err := a.Store.DB.Query(query, args...)
	if err != nil {
		fail(w, 500, "Ошибка чтения журнала")
		return
	}
	defer rows.Close()
	type event struct {
		ID        int64  `json:"id"`
		UserID    *int64 `json:"user_id"`
		FullName  string `json:"full_name"`
		GroupName string `json:"group_name"`
		IP        string `json:"ip"`
		Success   bool   `json:"success"`
		CreatedAt string `json:"created_at"`
	}
	out := []event{}
	for rows.Next() {
		var e event
		if err = rows.Scan(&e.ID, &e.UserID, &e.FullName, &e.GroupName, &e.IP, &e.Success, &e.CreatedAt); err != nil {
			fail(w, 500, "Ошибка чтения журнала")
			return
		}
		out = append(out, e)
	}
	var next int64
	if len(out) > 100 {
		out = out[:100]
		next = out[len(out)-1].ID
	}
	write(w, 200, map[string]any{"events": out, "next_before": next})
}
