package store

import (
	"database/sql"
	"errors"
	"fmt"
	"golang.org/x/text/unicode/norm"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// NormalizeIdentity is shared by migrations, roster imports and login.
func NormalizeIdentity(value string, limit int) (string, error) {
	if !utf8.ValidString(value) {
		return "", errors.New("Некорректный текст")
	}
	value = norm.NFC.String(strings.Join(strings.Fields(value), " "))
	value = strings.ToUpper(value)
	if utf8.RuneCountInString(value) == 0 || utf8.RuneCountInString(value) > limit {
		return "", fmt.Errorf("Значение должно содержать от 1 до %d символов", limit)
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", errors.New("Недопустимые невидимые символы")
		}
	}
	return value, nil
}

const identityTables = `
 CREATE TABLE IF NOT EXISTS groups(id INTEGER PRIMARY KEY,name TEXT NOT NULL UNIQUE,invite_required INTEGER NOT NULL DEFAULT 0 CHECK(invite_required IN(0,1)));
 CREATE TABLE IF NOT EXISTS invite_codes(id INTEGER PRIMARY KEY,group_id INTEGER NOT NULL REFERENCES groups(id),code TEXT NOT NULL UNIQUE,full_name TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 INSERT OR IGNORE INTO settings(key,value) VALUES('group_mode','free');
 CREATE TABLE IF NOT EXISTS staff_credentials(user_id INTEGER PRIMARY KEY REFERENCES users(id),login TEXT NOT NULL UNIQUE COLLATE NOCASE,password_hash TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS login_events(id INTEGER PRIMARY KEY,user_id INTEGER REFERENCES users(id),full_name TEXT NOT NULL,group_name TEXT NOT NULL,ip TEXT NOT NULL,success INTEGER NOT NULL,reason TEXT NOT NULL,kind TEXT NOT NULL,created_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS login_events_group_id ON login_events(group_name,id DESC);
 CREATE INDEX IF NOT EXISTS login_events_user_id ON login_events(user_id,id DESC);
`
const usersDDL = `CREATE TABLE users_identity_v2 (
 id INTEGER PRIMARY KEY,full_name TEXT NOT NULL,group_name TEXT NOT NULL DEFAULT '',
 invite_code_id INTEGER UNIQUE REFERENCES invite_codes(id),role TEXT NOT NULL CHECK(role IN('student','teacher','admin')),
 session_uuid TEXT,last_login_at TEXT NOT NULL DEFAULT '',last_login_ip TEXT NOT NULL DEFAULT '',
 blocked INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);`
const userIndexes = `CREATE UNIQUE INDEX IF NOT EXISTS users_student_identity ON users(full_name,group_name) WHERE role='student' AND invite_code_id IS NULL;
 CREATE TABLE IF NOT EXISTS sessions(id TEXT PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id),kind TEXT NOT NULL CHECK(kind IN('student','staff')),expires INTEGER NOT NULL,created_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS sessions_user_id ON sessions(user_id);
 PRAGMA user_version=2;`

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA secure_delete=ON; PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if err = s.migrateIdentity(path); err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS labs (id INTEGER PRIMARY KEY, title TEXT NOT NULL, description TEXT NOT NULL, position INTEGER NOT NULL, source TEXT NOT NULL DEFAULT 'custom');
 CREATE TABLE IF NOT EXISTS exercises (id INTEGER PRIMARY KEY, lab_id INTEGER NOT NULL REFERENCES labs(id), title TEXT NOT NULL, statement TEXT NOT NULL, template TEXT NOT NULL, tests TEXT NOT NULL, timeout_ms INTEGER NOT NULL DEFAULT 5000, memory_mb INTEGER NOT NULL DEFAULT 128, difficulty TEXT NOT NULL DEFAULT 'Базовый', position INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS submissions (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), exercise_id INTEGER NOT NULL REFERENCES exercises(id), code TEXT NOT NULL, mode TEXT NOT NULL, status TEXT NOT NULL, report TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')));
 CREATE INDEX IF NOT EXISTS submissions_user_exercise ON submissions(user_id,exercise_id,id DESC);
 CREATE TABLE IF NOT EXISTS drafts (user_id INTEGER NOT NULL REFERENCES users(id), exercise_id INTEGER NOT NULL REFERENCES exercises(id), code TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY(user_id,exercise_id));
 UPDATE submissions SET status='error',report='{"error":"Сервер был перезапущен. Отправьте решение повторно."}' WHERE status IN ('queued','running');`)
	if err != nil {
		db.Close()
		return nil, err
	}
	if err = s.migrateCatalog(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) migrateIdentity(path string) error {
	var exists int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&exists); err != nil {
		return err
	}
	legacy := false
	if exists > 0 {
		rows, err := s.DB.Query(`PRAGMA table_info(users)`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var cid, notnull, pk int
			var name, typ string
			var def any
			if err = rows.Scan(&cid, &name, &typ, &notnull, &def, &pk); err != nil {
				rows.Close()
				return err
			}
			if name == "password_hash" {
				legacy = true
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	if !legacy && exists > 0 {
		_, err := s.DB.Exec(identityTables + userIndexes)
		return err
	}
	if legacy {
		backup := path + ".before-identity-v2-" + time.Now().UTC().Format("20060102T150405.000000000") + ".bak"
		if _, err := s.DB.Exec(`VACUUM INTO ?`, backup); err != nil {
			return fmt.Errorf("identity backup: %w", err)
		}
		os.Chmod(backup, 0600)
	}
	if _, err := s.DB.Exec(`PRAGMA foreign_keys=OFF;`); err != nil {
		return err
	}
	defer s.DB.Exec(`PRAGMA foreign_keys=ON;`)
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(identityTables + usersDDL); err != nil {
		return err
	}
	if legacy {
		type oldUser struct {
			id                               int64
			login, name, hash, role, created string
			blocked                          bool
		}
		old := []oldUser{}
		rows, er := tx.Query(`SELECT id,login,name,password_hash,role,blocked,created_at FROM users ORDER BY id`)
		if er != nil {
			return er
		}
		for rows.Next() {
			var u oldUser
			if er = rows.Scan(&u.id, &u.login, &u.name, &u.hash, &u.role, &u.blocked, &u.created); er != nil {
				rows.Close()
				return er
			}
			old = append(old, u)
		}
		er = rows.Err()
		rows.Close()
		if er != nil {
			return er
		}
		seen := map[string]bool{}
		for _, u := range old {
			name, er := NormalizeIdentity(u.name, 150)
			if er != nil {
				name = fmt.Sprintf("ПОЛЬЗОВАТЕЛЬ %d", u.id)
			}
			group := ""
			if u.role == "student" {
				group = "АРХИВ"
				if seen[name] {
					runes := []rune(name)
					if len(runes) > 130 {
						runes = runes[:130]
					}
					name = fmt.Sprintf("%s (#%d)", string(runes), u.id)
				}
				seen[name] = true
				tx.Exec(`INSERT OR IGNORE INTO groups(name) VALUES(?)`, group)
			}
			if _, er = tx.Exec(`INSERT INTO users_identity_v2(id,full_name,group_name,role,blocked,created_at) VALUES(?,?,?,?,?,?)`, u.id, name, group, u.role, u.blocked, u.created); er != nil {
				return er
			}
			if u.role != "student" {
				if _, er = tx.Exec(`INSERT INTO staff_credentials(user_id,login,password_hash) VALUES(?,?,?)`, u.id, u.login, u.hash); er != nil {
					return er
				}
			}
		}
		if _, err = tx.Exec(`DROP TABLE IF EXISTS password_resets; DROP TABLE IF EXISTS sessions; DROP TABLE users;`); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`ALTER TABLE users_identity_v2 RENAME TO users;` + userIndexes); err != nil {
		return err
	}
	rows, err := tx.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	bad := rows.Next()
	rows.Close()
	if bad {
		return errors.New("identity migration: foreign key check failed")
	}
	return tx.Commit()
}

const UserFields = `u.id,COALESCE(c.login,''),u.full_name,u.group_name,u.role,u.blocked,u.created_at,u.invite_code_id,u.last_login_at,u.last_login_ip`

func scanUser(row interface{ Scan(...any) error }) (u User, err error) {
	err = row.Scan(&u.ID, &u.Login, &u.FullName, &u.GroupName, &u.Role, &u.Blocked, &u.Created, &u.InviteCodeID, &u.LastLoginAt, &u.LastLoginIP)
	u.Name = u.FullName
	return
}
func (s *Store) User(id int64) (User, error) {
	return scanUser(s.DB.QueryRow(`SELECT `+UserFields+` FROM users u LEFT JOIN staff_credentials c ON c.user_id=u.id WHERE u.id=?`, id))
}
func (s *Store) Users(studentsOnly bool) ([]User, error) {
	query := `SELECT ` + UserFields + ` FROM users u LEFT JOIN staff_credentials c ON c.user_id=u.id`
	if studentsOnly {
		query += ` WHERE u.role='student'`
	}
	rows, err := s.DB.Query(query + ` ORDER BY u.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, er := scanUser(rows)
		if er != nil {
			return nil, er
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

type Group struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	InviteRequired bool   `json:"invite_required"`
	Members        int    `json:"members,omitempty"`
	Codes          int    `json:"codes,omitempty"`
}

func (s *Store) Groups() ([]Group, error) {
	rows, err := s.DB.Query(`SELECT g.id,g.name,g.invite_required,(SELECT COUNT(*) FROM users u WHERE u.role='student' AND u.group_name=g.name),(SELECT COUNT(*) FROM invite_codes i WHERE i.group_id=g.id) FROM groups g ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		if err = rows.Scan(&g.ID, &g.Name, &g.InviteRequired, &g.Members, &g.Codes); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
