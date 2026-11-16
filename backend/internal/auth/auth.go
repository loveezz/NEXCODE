package auth

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
	"net/http"
	"nexcode/internal/store"
	"strings"
	"time"
	_ "time/tzdata"
)

var ErrDenied = errors.New("Не удалось войти. Проверьте ФИО, группу и код.")

type Service struct {
	Store    *store.Store
	Secure   bool
	Logger   *slog.Logger
	Location *time.Location
}

func Random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func HashPassword(p string) (string, error) {
	if len(p) < 10 || len(p) > 72 {
		return "", errors.New("Пароль должен содержать от 10 до 72 байт")
	}
	b, e := bcrypt.GenerateFromPassword([]byte(p), 12)
	return string(b), e
}
func (s *Service) location() *time.Location {
	if s.Location != nil {
		return s.Location
	}
	return time.UTC
}
func (s *Service) DefaultSemesterEnd() string {
	now := time.Now().In(s.location())
	year := now.Year()
	month, day := time.January, 31
	if now.Month() == time.January {
	} else if now.Month() <= time.June {
		month, day = time.June, 30
	} else {
		year++
	}
	return time.Date(year, month, day, 0, 0, 0, 0, s.location()).Format("2006-01-02")
}
func (s *Service) SemesterExpiry(date string) (time.Time, error) {
	v, err := time.ParseInLocation("2006-01-02", date, s.location())
	if err != nil {
		return time.Time{}, errors.New("Укажите дату окончания семестра")
	}
	return v.AddDate(0, 0, 1).Add(-time.Second), nil
}
func (s *Service) InitSemester(configured string) error {
	if configured == "" {
		configured = s.DefaultSemesterEnd()
	}
	if _, err := s.SemesterExpiry(configured); err != nil {
		return err
	}
	_, err := s.Store.DB.Exec(`INSERT OR IGNORE INTO settings(key,value) VALUES('semester_end',?)`, configured)
	return err
}
func (s *Service) SemesterEnd() (string, error) {
	var date string
	err := s.Store.DB.QueryRow(`SELECT value FROM settings WHERE key='semester_end'`).Scan(&date)
	return date, err
}
func (s *Service) cookie(w http.ResponseWriter, id string, expiry time.Time) {
	http.SetCookie(w, &http.Cookie{Name: "nexcode_session", Value: id, Path: "/", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteLaxMode, Expires: expiry.UTC(), MaxAge: int(time.Until(expiry).Seconds())})
}
func (s *Service) issueTx(tx *sql.Tx, uid int64, kind, ip string) (string, time.Time, error) {
	expiry := time.Now().Add(24 * time.Hour)
	if kind == "student" {
		var date string
		if err := tx.QueryRow(`SELECT value FROM settings WHERE key='semester_end'`).Scan(&date); err != nil {
			return "", expiry, err
		}
		var err error
		expiry, err = s.SemesterExpiry(date)
		if err != nil {
			return "", expiry, err
		}
		if !expiry.After(time.Now()) {
			return "", expiry, ErrDenied
		}
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return "", expiry, err
	}
	now := store.Now()
	if _, err = tx.Exec(`DELETE FROM sessions WHERE expires<?`, time.Now().Unix()); err != nil {
		return "", expiry, err
	}
	if _, err = tx.Exec(`INSERT INTO sessions(id,user_id,kind,expires,created_at) VALUES(?,?,?,?,?)`, id.String(), uid, kind, expiry.Unix(), now); err != nil {
		return "", expiry, err
	}
	if _, err = tx.Exec(`UPDATE users SET session_uuid=?,last_login_at=?,last_login_ip=? WHERE id=?`, id.String(), now, ip, uid); err != nil {
		return "", expiry, err
	}
	if _, err = tx.Exec(`INSERT INTO login_events(user_id,full_name,group_name,ip,success,reason,kind,created_at) SELECT id,full_name,group_name,?,1,'success',?,? FROM users WHERE id=?`, ip, kind, now, uid); err != nil {
		return "", expiry, err
	}
	return id.String(), expiry, nil
}
func (s *Service) StudentLogin(w http.ResponseWriter, name, group, code, ip string) (store.User, error) {
	tx, err := s.Store.DB.Begin()
	if err != nil {
		return store.User{}, err
	}
	defer tx.Rollback()
	var mode string
	if err = tx.QueryRow(`SELECT value FROM settings WHERE key='group_mode'`).Scan(&mode); err != nil {
		return store.User{}, err
	}
	var gid int64
	var required bool
	err = tx.QueryRow(`SELECT id,invite_required FROM groups WHERE name=?`, group).Scan(&gid, &required)
	if errors.Is(err, sql.ErrNoRows) {
		if mode != "free" {
			return store.User{}, ErrDenied
		}
		res, er := tx.Exec(`INSERT INTO groups(name) VALUES(?)`, group)
		if er != nil {
			return store.User{}, er
		}
		gid, _ = res.LastInsertId()
	} else if err != nil {
		return store.User{}, err
	}
	var uid int64
	var blocked bool
	if required {
		code = strings.ToUpper(strings.TrimSpace(code))
		if !ValidCode(code) {
			return store.User{}, ErrDenied
		}
		var invite int64
		if err = tx.QueryRow(`SELECT id FROM invite_codes WHERE group_id=? AND code=? AND full_name=?`, gid, code, name).Scan(&invite); errors.Is(err, sql.ErrNoRows) {
			return store.User{}, ErrDenied
		} else if err != nil {
			return store.User{}, err
		}
		err = tx.QueryRow(`SELECT id,blocked FROM users WHERE invite_code_id=? AND role='student'`, invite).Scan(&uid, &blocked)
		if errors.Is(err, sql.ErrNoRows) {
			// Adopt one existing passwordless profile, preserving its drafts and submissions.
			err = tx.QueryRow(`SELECT id,blocked FROM users WHERE full_name=? AND group_name=? AND role='student' AND invite_code_id IS NULL`, name, group).Scan(&uid, &blocked)
			if err == nil {
				_, err = tx.Exec(`UPDATE users SET invite_code_id=? WHERE id=?`, invite, uid)
			} else if errors.Is(err, sql.ErrNoRows) {
				var res sql.Result
				res, err = tx.Exec(`INSERT INTO users(full_name,group_name,invite_code_id,role) VALUES(?,?,?,'student')`, name, group, invite)
				if err == nil {
					uid, _ = res.LastInsertId()
				}
			}
		}
	} else {
		err = tx.QueryRow(`SELECT id,blocked FROM users WHERE full_name=? AND group_name=? AND role='student' ORDER BY id LIMIT 1`, name, group).Scan(&uid, &blocked)
		if errors.Is(err, sql.ErrNoRows) {
			var res sql.Result
			res, err = tx.Exec(`INSERT INTO users(full_name,group_name,role) VALUES(?,?,'student')`, name, group)
			if err == nil {
				uid, _ = res.LastInsertId()
			}
		}
	}
	if err != nil {
		return store.User{}, err
	}
	if blocked {
		return store.User{}, ErrDenied
	}
	sid, expiry, err := s.issueTx(tx, uid, "student", ip)
	if err != nil {
		return store.User{}, err
	}
	if err = tx.Commit(); err != nil {
		return store.User{}, err
	}
	u, err := s.Store.User(uid)
	if err != nil {
		return store.User{}, err
	}
	s.cookie(w, sid, expiry)
	s.log(name, group, ip, "success", "student")
	return u, nil
}

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("nexcode-timing-placeholder"), 12)

func (s *Service) StaffLogin(w http.ResponseWriter, login, password, ip string) (store.User, error) {
	var uid int64
	var hash string
	var blocked bool
	err := s.Store.DB.QueryRow(`SELECT u.id,c.password_hash,u.blocked FROM staff_credentials c JOIN users u ON u.id=c.user_id WHERE c.login=? AND u.role IN('teacher','admin')`, strings.TrimSpace(login)).Scan(&uid, &hash, &blocked)
	if errors.Is(err, sql.ErrNoRows) {
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return store.User{}, ErrDenied
	}
	if err != nil {
		return store.User{}, err
	}
	if len(password) > 72 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || blocked {
		return store.User{}, ErrDenied
	}
	tx, err := s.Store.DB.Begin()
	if err != nil {
		return store.User{}, err
	}
	defer tx.Rollback()
	sid, expiry, err := s.issueTx(tx, uid, "staff", ip)
	if err != nil {
		return store.User{}, err
	}
	if err = tx.Commit(); err != nil {
		return store.User{}, err
	}
	u, err := s.Store.User(uid)
	if err != nil {
		return u, err
	}
	s.cookie(w, sid, expiry)
	s.log(u.FullName, "", ip, "success", "staff")
	return u, nil
}
func (s *Service) Read(r *http.Request) (store.User, string, error) {
	c, err := r.Cookie("nexcode_session")
	if err != nil {
		return store.User{}, "", err
	}
	parsed, err := uuid.Parse(c.Value)
	if err != nil || parsed.Version() != 4 || parsed.String() != c.Value {
		return store.User{}, "", ErrDenied
	}
	var uid int64
	var kind string
	err = s.Store.DB.QueryRow(`SELECT user_id,kind FROM sessions WHERE id=? AND expires>?`, c.Value, time.Now().Unix()).Scan(&uid, &kind)
	if err != nil {
		return store.User{}, "", err
	}
	u, err := s.Store.User(uid)
	if err != nil {
		return u, "", err
	}
	if u.Blocked || (kind == "student" && u.Role != "student") || (kind == "staff" && u.Role == "student") {
		return store.User{}, "", ErrDenied
	}
	return u, c.Value, nil
}
func (s *Service) Refresh(w http.ResponseWriter, id string) {
	var expiry int64
	if s.Store.DB.QueryRow(`SELECT expires FROM sessions WHERE id=? AND kind='student'`, id).Scan(&expiry) == nil {
		s.cookie(w, id, time.Unix(expiry, 0))
	}
}
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("nexcode_session"); err == nil {
		s.Store.DB.Exec(`DELETE FROM sessions WHERE id=?`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "nexcode_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteLaxMode})
}
func (s *Service) log(name, group, ip, result, kind string) {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("login", "ip", ip, "full_name", name, "group", group, "result", result, "kind", kind)
}
func (s *Service) Denied(name, group, ip, reason, kind string) {
	s.log(name, group, ip, "denied", kind)
	var uid any
	_, err := s.Store.DB.Exec(`INSERT INTO login_events(user_id,full_name,group_name,ip,success,reason,kind,created_at) VALUES(?,?,?,?,0,?,?,?)`, uid, name, group, ip, reason, kind, store.Now())
	if err != nil {
		slog.Error("login audit persistence failed")
	}
}
func ValidCode(code string) bool {
	if len(code) < 4 || len(code) > 8 {
		return false
	}
	for _, r := range code {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

type StaffAccount struct {
	Login        string `json:"login"`
	FullName     string `json:"full_name"`
	Role         string `json:"role"`
	PasswordHash string `json:"password_hash"`
}

func (s *Service) ConfigureStaff(data []byte) error {
	var accounts []StaffAccount
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&accounts); err != nil {
		return err
	}
	if len(accounts) > 100 {
		return errors.New("Too many staff accounts")
	}
	tx, err := s.Store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, account := range accounts {
		account.Login = strings.ToLower(strings.TrimSpace(account.Login))
		name, er := store.NormalizeIdentity(account.FullName, 150)
		if er != nil {
			return er
		}
		if account.Login == "" || len(account.Login) > 100 || strings.ContainsAny(account.Login, " \t\r\n") || (account.Role != "teacher" && account.Role != "admin") {
			return errors.New("Invalid staff account")
		}
		cost, costErr := bcrypt.Cost([]byte(account.PasswordHash))
		if costErr != nil || cost < 10 || cost > 16 {
			return errors.New("Staff password_hash must be bcrypt")
		}
		var uid int64
		var oldHash, role string
		er = tx.QueryRow(`SELECT c.user_id,c.password_hash,u.role FROM staff_credentials c JOIN users u ON u.id=c.user_id WHERE c.login=?`, account.Login).Scan(&uid, &oldHash, &role)
		if errors.Is(er, sql.ErrNoRows) {
			res, e := tx.Exec(`INSERT INTO users(full_name,role) VALUES(?,?)`, name, account.Role)
			if e != nil {
				return e
			}
			uid, _ = res.LastInsertId()
			_, er = tx.Exec(`INSERT INTO staff_credentials(user_id,login,password_hash) VALUES(?,?,?)`, uid, account.Login, account.PasswordHash)
		} else if er == nil {
			_, er = tx.Exec(`UPDATE staff_credentials SET password_hash=? WHERE user_id=?`, account.PasswordHash, uid)
			if er == nil {
				_, er = tx.Exec(`UPDATE users SET full_name=?,role=? WHERE id=?`, name, account.Role, uid)
			}
			if er == nil && (oldHash != account.PasswordHash || role != account.Role) {
				_, er = tx.Exec(`DELETE FROM sessions WHERE user_id=?`, uid)
			}
		}
		if er != nil {
			return er
		}
	}
	var admins int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin' AND blocked=0`).Scan(&admins); err != nil {
		return err
	}
	if len(accounts) > 0 && admins == 0 {
		return errors.New("Configuration must retain an active administrator")
	}
	return tx.Commit()
}
