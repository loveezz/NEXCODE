package store

import (
	"path/filepath"
	"testing"
)

func TestPracticeReplacesCatalogWithoutLosingStudentWork(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	s.DB.Exec(`INSERT INTO labs(id,title,description,position,source) VALUES(1,'Old demo','',1,'demo')`)
	s.DB.Exec(`INSERT INTO exercises(id,lab_id,title,statement,template,tests) VALUES(1,1,'Old exercise','old','','')`)
	s.DB.Exec(`INSERT INTO users(id,full_name,group_name,role) VALUES(1,'STUDENT','G','student')`)
	s.DB.Exec(`INSERT INTO drafts(user_id,exercise_id,code,updated_at) VALUES(1,1,'saved draft','now')`)
	s.DB.Exec(`INSERT INTO submissions(user_id,exercise_id,code,mode,status) VALUES(1,1,'saved solution','check','passed')`)
	if err = s.Seed(); err != nil {
		t.Fatal(err)
	}
	labs, _ := s.Labs()
	exercises, _ := s.Exercises(1)
	if len(labs) != 4 || len(exercises) != 72 {
		t.Fatalf("active catalog: %d/%d", len(labs), len(exercises))
	}
	if _, err = s.Exercise(1); err == nil {
		t.Fatal("archived exercise remains available")
	}
	for _, lab := range labs {
		if lab.Source != "practice" || lab.DocumentURL == "" {
			t.Fatal("source document missing")
		}
	}
	var draft, submission string
	s.DB.QueryRow(`SELECT code FROM drafts WHERE user_id=1 AND exercise_id=1`).Scan(&draft)
	s.DB.QueryRow(`SELECT code FROM submissions WHERE user_id=1 AND exercise_id=1`).Scan(&submission)
	if draft != "saved draft" || submission != "saved solution" {
		t.Fatal("student history lost")
	}
	first := exercises[0].ID
	if err = s.Seed(); err != nil {
		t.Fatal(err)
	}
	again, _ := s.Exercises(1)
	if len(again) != 72 || again[0].ID != first {
		t.Fatal("restart duplicated curriculum or changed IDs")
	}
	var issues int
	rows, err := s.DB.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		issues++
	}
	if issues != 0 {
		t.Fatal("broken foreign keys")
	}
}
