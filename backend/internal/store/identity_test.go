package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyMigrationPreservesIDsHistoryAndMovesCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY,login TEXT,name TEXT,password_hash TEXT,role TEXT,blocked INTEGER,created_at TEXT);
 INSERT INTO users VALUES(7,'admin','Admin','staff-bcrypt','admin',0,'2026-01-01');
 INSERT INTO users VALUES(42,'one','Иванов Иван','old-student-password','student',0,'2026-01-01');
 INSERT INTO users VALUES(43,'two','Иванов Иван','old-student-password','student',0,'2026-01-01');
 CREATE TABLE labs(id INTEGER PRIMARY KEY,title TEXT,description TEXT,position INTEGER,source TEXT);INSERT INTO labs VALUES(1,'Lab','',1,'demo');
 CREATE TABLE exercises(id INTEGER PRIMARY KEY,lab_id INTEGER,title TEXT,statement TEXT,template TEXT,tests TEXT,timeout_ms INTEGER,memory_mb INTEGER,difficulty TEXT,position INTEGER);INSERT INTO exercises VALUES(1,1,'Task','s','','{}',5000,128,'basic',1);
 CREATE TABLE submissions(id INTEGER PRIMARY KEY,user_id INTEGER REFERENCES users(id),exercise_id INTEGER,code TEXT,mode TEXT,status TEXT,report TEXT,created_at TEXT);INSERT INTO submissions VALUES(91,42,1,'original code','check','passed','{}','2026-01-01');
 CREATE TABLE drafts(user_id INTEGER REFERENCES users(id),exercise_id INTEGER,code TEXT,updated_at TEXT,PRIMARY KEY(user_id,exercise_id));INSERT INTO drafts VALUES(42,1,'draft code','2026-01-01');
 CREATE TABLE sessions(id TEXT PRIMARY KEY,user_id INTEGER,expires INTEGER);INSERT INTO sessions VALUES('old-jwt-id',42,9999999999);
 CREATE TABLE password_resets(token_hash TEXT,user_id INTEGER,expires INTEGER);`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	u, err := s.User(42)
	if err != nil || u.GroupName != "АРХИВ" || u.FullName != "ИВАНОВ ИВАН" {
		t.Fatalf("migration identity: %+v %v", u, err)
	}
	other, _ := s.User(43)
	if other.FullName == u.FullName {
		t.Fatal("legacy duplicates merged")
	}
	var hash, code string
	s.DB.QueryRow(`SELECT password_hash FROM staff_credentials WHERE user_id=7`).Scan(&hash)
	if hash != "staff-bcrypt" {
		t.Fatal("staff credential lost")
	}
	s.DB.QueryRow(`SELECT code FROM submissions WHERE id=91 AND user_id=42`).Scan(&code)
	if code != "original code" {
		t.Fatal("submission lost")
	}
	s.DB.QueryRow(`SELECT code FROM drafts WHERE user_id=42`).Scan(&code)
	if code != "draft code" {
		t.Fatal("draft lost")
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n)
	if n != 0 {
		t.Fatal("legacy sessions retained")
	}
	rows, _ := s.DB.Query(`PRAGMA table_info(users)`)
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var def any
		rows.Scan(&cid, &name, &typ, &notnull, &def, &pk)
		if name == "password_hash" || name == "email" {
			t.Fatal("sensitive legacy field retained")
		}
	}
	rows.Close()
	backups, _ := filepath.Glob(path + ".before-identity-v2-*.bak")
	if len(backups) != 1 {
		t.Fatal("missing pre-migration backup")
	}
	s.DB.Close()
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	again.DB.Close()
}
func TestNormalizeIdentity(t *testing.T) {
	normalized, err := NormalizeIdentity("  Иванов\u00a0 Иван  ", 150)
	if err != nil || normalized != "ИВАНОВ ИВАН" {
		t.Fatal(normalized, err)
	}
	if _, err = NormalizeIdentity(strings.Repeat("Я", 150), 150); err != nil {
		t.Fatal("counts bytes instead of characters")
	}
	if _, err = NormalizeIdentity("Ива\u200bнов", 150); err == nil {
		t.Fatal("invisible duplicate allowed")
	}
}
