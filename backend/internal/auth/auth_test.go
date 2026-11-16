package auth

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"nexcode/internal/store"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentLoginsKeepOneIdentityAndManySessions(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	var logs bytes.Buffer
	a := &Service{Store: s, Secure: true, Logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	a.InitSemester("")
	var wg sync.WaitGroup
	cookies := make(chan string, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			_, er := a.StudentLogin(w, "ИВАНОВ ИВАН", "ПИ-21", "", "192.0.2.1")
			if er != nil {
				t.Error(er)
				return
			}
			cookie := w.Result().Cookies()[0]
			if !cookie.Secure {
				t.Error("missing Secure")
			}
			cookies <- cookie.Value
		}()
	}
	wg.Wait()
	close(cookies)
	unique := map[string]bool{}
	for value := range cookies {
		unique[value] = true
	}
	var users, sessions int
	s.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	s.DB.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessions)
	if users != 1 || sessions != 24 || len(unique) != 24 {
		t.Fatalf("users=%d sessions=%d unique=%d", users, sessions, len(unique))
	}
	if !strings.Contains(logs.String(), `"result":"success"`) {
		t.Fatal("missing technical audit")
	}
	for value := range unique {
		if strings.Contains(logs.String(), value) {
			t.Fatal("session credential leaked into audit")
		}
	}
}
func TestExpiredSemesterAndSessionDenied(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "expiry.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	a := &Service{Store: s}
	a.InitSemester("")
	w := httptest.NewRecorder()
	_, err = a.StudentLogin(w, "TEST STUDENT", "TEST", "", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	cookie := w.Result().Cookies()[0]
	s.DB.Exec(`UPDATE sessions SET expires=?`, time.Now().Add(-time.Second).Unix())
	request := httptest.NewRequest("GET", "/", nil)
	request.AddCookie(cookie)
	if _, _, err = a.Read(request); err == nil {
		t.Fatal("expired session accepted")
	}
	s.DB.Exec(`UPDATE settings SET value='2000-01-01' WHERE key='semester_end'`)
	if _, err = a.StudentLogin(httptest.NewRecorder(), "OTHER STUDENT", "TEST", "", "192.0.2.1"); err != ErrDenied {
		t.Fatal("expired semester accepted", err)
	}
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE full_name='OTHER STUDENT'`).Scan(&count)
	if count != 0 {
		t.Fatal("failed login created an orphan profile")
	}
}
func TestStaffConfigurationBcryptOnlyAndLastAdmin(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "staff.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	a := &Service{Store: s}
	hash, err := HashPassword("test-staff-password-123")
	if err != nil {
		t.Fatal(err)
	}
	accounts := []StaffAccount{{Login: "admin", FullName: "Admin", Role: "admin", PasswordHash: hash}}
	data, _ := json.Marshal(accounts)
	if err = a.ConfigureStaff(data); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	u, err := a.StaffLogin(w, "admin", "test-staff-password-123", "192.0.2.1")
	if err != nil || u.Role != "admin" {
		t.Fatal(err)
	}
	accounts[0].Role = "teacher"
	data, _ = json.Marshal(accounts)
	if err = a.ConfigureStaff(data); err == nil {
		t.Fatal("last administrator removed")
	}
	current, _ := s.User(u.ID)
	if current.Role != "admin" {
		t.Fatal("failed configuration was committed")
	}
	newHash, _ := HashPassword("new-staff-password-456")
	accounts[0].Role = "admin"
	accounts[0].PasswordHash = newHash
	data, _ = json.Marshal(accounts)
	if err = a.ConfigureStaff(data); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.AddCookie(w.Result().Cookies()[0])
	if _, _, err = a.Read(request); err == nil {
		t.Fatal("password rotation retained staff session")
	}
	if err = a.ConfigureStaff([]byte(`[{"login":"x","full_name":"User","role":"admin","password_hash":"plaintext"}]`)); err == nil {
		t.Fatal("plaintext credential accepted")
	}
}
