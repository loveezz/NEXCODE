package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"nexcode/internal/auth"
	"nexcode/internal/engine"
	"nexcode/internal/store"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeExecutor struct{}

func (fakeExecutor) Execute(context.Context, engine.Request) (engine.Response, error) {
	return engine.Response{}, nil
}

var testHash, _ = bcrypt.GenerateFromPassword([]byte("test-password-123"), 10)

func setup(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Seed(); err != nil {
		t.Fatal(err)
	}
	a := &auth.Service{Store: s, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	if err = a.InitSemester(""); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal([]auth.StaffAccount{{Login: "admin", FullName: "Администратор", Role: "admin", PasswordHash: string(testHash)}, {Login: "teacher", FullName: "Преподаватель", Role: "teacher", PasswordHash: string(testHash)}})
	if err = a.ConfigureStaff(data); err != nil {
		t.Fatal(err)
	}
	q := engine.NewQueue(s, fakeExecutor{}, 1, 4)
	t.Cleanup(func() { q.Close(); s.DB.Close() })
	return New(s, a, q, "http://localhost:3000"), s
}
func request(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	return requestIP(h, method, path, body, cookie, "192.0.2.10:1000")
}
func requestIP(h http.Handler, method, path, body string, cookie *http.Cookie, address string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.RemoteAddr = address
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func student(t *testing.T, h http.Handler, name, group, code string) (store.User, *http.Cookie) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"full_name": name, "group_name": group, "code": code})
	w := request(h, "POST", "/api/auth/login", string(body), nil)
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	var u store.User
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}
	cookie := w.Result().Cookies()[0]
	parsed, err := uuid.Parse(cookie.Value)
	if err != nil || parsed.Version() != 4 || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge <= 86400 {
		t.Fatalf("invalid cookie: %+v", cookie)
	}
	return u, cookie
}
func staff(t *testing.T, h http.Handler, login string) *http.Cookie {
	t.Helper()
	w := requestIP(h, "POST", "/api/auth/staff/login", `{"login":"`+login+`","password":"test-password-123"}`, nil, "192.0.2.20:1000")
	if w.Code != 200 {
		t.Fatalf("staff login: %d %s", w.Code, w.Body)
	}
	return w.Result().Cookies()[0]
}
func TestIdentityNormalizationMultiDeviceAndLogout(t *testing.T) {
	h, s := setup(t)
	u, first := student(t, h, "  Иванов  Иван\tИванович ", " пи-21 ", "")
	again, second := student(t, h, "ИВАНОВ ИВАН ИВАНОВИЧ", "ПИ-21", "")
	if u.ID != again.ID || u.FullName != "ИВАНОВ ИВАН ИВАНОВИЧ" || first.Value == second.Value {
		t.Fatal("identity or session mismatch")
	}
	if w := request(h, "PUT", "/api/exercises/1/draft", `{"code":"print(42)"}`, first); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w := request(h, "GET", "/api/exercises/1/draft", "", second); !strings.Contains(w.Body.String(), "print(42)") {
		t.Fatal("draft did not follow identity")
	}
	request(h, "POST", "/api/auth/logout", `{}`, first)
	if w := request(h, "GET", "/api/catalog", "", first); w.Code != 401 {
		t.Fatal("logged-out session accepted")
	}
	if w := request(h, "GET", "/api/catalog", "", second); w.Code != 200 {
		t.Fatal("another device logged out")
	}
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE full_name=? AND group_name=?`, u.FullName, u.GroupName).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate user")
	}
	var at, ip string
	s.DB.QueryRow(`SELECT last_login_at,last_login_ip FROM users WHERE id=?`, u.ID).Scan(&at, &ip)
	if at == "" || ip != "192.0.2.10" {
		t.Fatal("missing audit metadata")
	}
}
func TestValidationRolesAndCSRF(t *testing.T) {
	h, _ := setup(t)
	for _, body := range []string{`{"full_name":" ","group_name":"a"}`, `{"full_name":"a","group_name":"` + strings.Repeat("Я", 21) + `"}`, `{"full_name":"` + strings.Repeat("Я", 151) + `","group_name":"a"}`, `{"full_name":"a","group_name":"b","role":"admin"}`} {
		if w := request(h, "POST", "/api/auth/login", body, nil); w.Code != 400 {
			t.Fatalf("accepted invalid input: %s", w.Body)
		}
	}
	u, cookie := student(t, h, strings.Repeat("Я", 150), strings.Repeat("Г", 20), "")
	if u.Role != "student" {
		t.Fatal("role elevated")
	}
	if w := request(h, "GET", "/api/teacher/identity", "", cookie); w.Code != 403 {
		t.Fatal("student read teacher settings")
	}
	r := httptest.NewRequest("PUT", "/api/exercises/1/draft", strings.NewReader(`{"code":"x"}`))
	r.AddCookie(cookie)
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("CSRF accepted")
	}
	if w = request(h, "GET", "/api/catalog", "", cookie); strings.Contains(w.Body.String(), `"tests":`) {
		t.Fatal("tests exposed")
	}
	for _, path := range []string{"/api/auth/register", "/api/auth/forgot", "/api/auth/reset"} {
		if w = request(h, "POST", path, `{}`, nil); w.Code != 404 {
			t.Fatalf("legacy endpoint survived: %s", path)
		}
	}
}
func TestInvitesNamesakesUniformDenialsAndAdoption(t *testing.T) {
	h, s := setup(t)
	old, cookie := student(t, h, "Иванов Иван Иванович", "ИТ-1", "")
	request(h, "PUT", "/api/exercises/1/draft", `{"code":"my draft"}`, cookie)
	admin := staff(t, h, "admin")
	var gid int64
	s.DB.QueryRow(`SELECT id FROM groups WHERE name='ИТ-1'`).Scan(&gid)
	w := request(h, "POST", fmt.Sprintf("/api/teacher/groups/%d/invites", gid), `{"students":[{"full_name":"Иванов Иван Иванович","code":"AB12CD"},{"full_name":"Иванов Иван Иванович","code":"XY34EF"}]}`, admin)
	if w.Code != 201 {
		t.Fatal(w.Body)
	}
	w = request(h, "PUT", fmt.Sprintf("/api/teacher/groups/%d", gid), `{"name":"ИТ-1","invite_required":true}`, admin)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w = request(h, "GET", "/api/catalog", "", cookie); w.Code != 401 {
		t.Fatal("unverified session survived code requirement")
	}
	adopted, first := student(t, h, "Иванов Иван Иванович", "ИТ-1", "ab12cd")
	if adopted.ID != old.ID {
		t.Fatal("progress identity not preserved")
	}
	namesake, second := student(t, h, "Иванов Иван Иванович", "ИТ-1", "XY34EF")
	if namesake.ID == old.ID {
		t.Fatal("namesakes merged")
	}
	if w = request(h, "GET", "/api/exercises/1/draft", "", first); !strings.Contains(w.Body.String(), "my draft") {
		t.Fatal("draft lost")
	}
	if w = request(h, "GET", "/api/exercises/1/draft", "", second); strings.Contains(w.Body.String(), "my draft") {
		t.Fatal("draft leaked")
	}
	one := request(h, "POST", "/api/auth/login", `{"full_name":"Иванов Иван Иванович","group_name":"ИТ-1","code":"NOPE"}`, nil)
	two := request(h, "POST", "/api/auth/login", `{"full_name":"Неизвестное Имя","group_name":"ИТ-1","code":"AB12CD"}`, nil)
	if one.Code != 401 || two.Code != 401 || one.Body.String() != two.Body.String() {
		t.Fatal("credential enumeration")
	}
	if w = request(h, "PUT", fmt.Sprintf("/api/teacher/groups/%d", gid), `{"name":"ИТ-1","invite_required":false}`, admin); w.Code != 409 {
		t.Fatal("ambiguous passwordless group allowed")
	}
}
func TestTeacherSemesterAndSessionClear(t *testing.T) {
	h, s := setup(t)
	teacher := staff(t, h, "teacher")
	u, cookie := student(t, h, "Сидоров Пётр", "ПИ-22", "")
	target := time.Now().AddDate(0, 0, 30).UTC().Format("2006-01-02")
	body := fmt.Sprintf(`{"group_mode":"list","semester_end":"%s"}`, target)
	w := request(h, "PUT", "/api/teacher/identity", body, teacher)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var expires int64
	s.DB.QueryRow(`SELECT expires FROM sessions WHERE id=?`, cookie.Value).Scan(&expires)
	if time.Unix(expires, 0).UTC().Format("2006-01-02") != target {
		t.Fatal("semester change not applied")
	}
	if w = request(h, "POST", "/api/auth/login", `{"full_name":"Имя","group_name":"НЕИЗВЕСТНАЯ"}`, nil); w.Code != 401 {
		t.Fatal("unknown group allowed")
	}
	if w = request(h, "DELETE", fmt.Sprintf("/api/teacher/students/%d/sessions", u.ID), "", teacher); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w = request(h, "GET", "/api/catalog", "", cookie); w.Code != 401 {
		t.Fatal("cleared session accepted")
	}
	if w = request(h, "GET", "/api/teacher/login-events?group=ПИ-22", "", teacher); w.Code != 200 || !strings.Contains(w.Body.String(), u.FullName) {
		t.Fatal("teacher audit unavailable")
	}
}
func TestLoginRateLimitAndRoleBoundary(t *testing.T) {
	h, _ := setup(t)
	for i := 0; i < 10; i++ {
		body := fmt.Sprintf(`{"full_name":"Студент %d","group_name":"RATE"}`, i)
		if w := request(h, "POST", "/api/auth/login", body, nil); w.Code != 200 {
			t.Fatal(w.Body)
		}
	}
	if w := request(h, "POST", "/api/auth/login", `{"full_name":"Имя","group_name":"RATE"}`, nil); w.Code != 429 {
		t.Fatal("rate limit missing")
	}
	admin := staff(t, h, "admin")
	var users []store.User
	w := request(h, "GET", "/api/admin/users", "", admin)
	json.Unmarshal(w.Body.Bytes(), &users)
	for _, u := range users {
		if u.Role == "student" {
			if w = request(h, "PATCH", fmt.Sprintf("/api/admin/users/%d", u.ID), `{"role":"admin","blocked":false}`, admin); w.Code != 400 {
				t.Fatal("passwordless identity promoted")
			}
			break
		}
	}
}
func TestBlockedIdentityAndAudit(t *testing.T) {
	h, s := setup(t)
	u, cookie := student(t, h, "Студент Журнал", "LOG", "")
	s.DB.Exec(`UPDATE users SET blocked=1 WHERE id=?`, u.ID)
	if w := request(h, "GET", "/api/auth/me", "", cookie); w.Code != 401 {
		t.Fatal("blocked session accepted")
	}
	if w := request(h, "POST", "/api/auth/login", `{"full_name":"Студент Журнал","group_name":"LOG"}`, nil); w.Code != 401 {
		t.Fatal("blocked identity logged in")
	}
	var success, denied int
	s.DB.QueryRow(`SELECT COUNT(*) FROM login_events WHERE success=1 AND user_id=?`, u.ID).Scan(&success)
	s.DB.QueryRow(`SELECT COUNT(*) FROM login_events WHERE success=0 AND full_name=?`, u.FullName).Scan(&denied)
	if success != 1 || denied != 1 {
		t.Fatalf("audit: success=%d denied=%d", success, denied)
	}
}
func TestClientIPOnlyTrustsConfiguredProxy(t *testing.T) {
	h, s := setup(t)
	_ = s
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"full_name":"Иванов Иван","group_name":"IP"}`))
	r.RemoteAddr = "192.0.2.30:1000"
	r.Header.Set("X-Forwarded-For", "198.51.100.99")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var u store.User
	json.Unmarshal(w.Body.Bytes(), &u)
	if u.LastLoginIP != "192.0.2.30" {
		t.Fatal("untrusted forwarded IP accepted")
	}
}
func TestImportAtomicAndHistoryProtected(t *testing.T) {
	h, s := setup(t)
	teacher := staff(t, h, "teacher")
	u, _ := student(t, h, "Студент Тест", "TEST", "")
	body := `{"labs":[{"title":"Valid lab","description":"","position":5,"exercises":[{"title":"Invalid tests","statement":"test","template":"","tests":"bad json"}]}]}`
	if w := request(h, "POST", "/api/teacher/import", body, teacher); w.Code != 400 {
		t.Fatal(w.Body)
	}
	labs, _ := s.Labs()
	if len(labs) != 4 {
		t.Fatal("partial import committed")
	}
	s.DB.Exec(`INSERT INTO submissions(user_id,exercise_id,code,mode,status) VALUES(?,1,'x','check','passed')`, u.ID)
	if w := request(h, "DELETE", "/api/teacher/exercises/1", "", teacher); w.Code != 409 {
		t.Fatal("history deleted")
	}
}
