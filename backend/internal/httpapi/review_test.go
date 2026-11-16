package httpapi

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestWrittenWorkNeedsTeacherReviewAndPreservesFeedback(t *testing.T) {
	h, s := setup(t)
	studentUser, cookie := student(t, h, "Автор Ответа", "REVIEW", "")
	teacher := staff(t, h, "teacher")
	var eid int64
	s.DB.QueryRow(`SELECT id FROM exercises WHERE kind='written' ORDER BY id LIMIT 1`).Scan(&eid)
	url := fmt.Sprintf("/api/exercises/%d/submit", eid)
	w := request(h, "POST", url, `{"code":"Мой алгоритм из семи шагов","mode":"check"}`, cookie)
	if w.Code != 202 {
		t.Fatal(w.Body)
	}
	var job struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	json.Unmarshal(w.Body.Bytes(), &job)
	if job.Status != "review" {
		t.Fatal("written work automatically graded")
	}
	reviewURL := fmt.Sprintf("/api/teacher/submissions/%d/review", job.ID)
	if w = request(h, "PATCH", reviewURL, `{"status":"passed","feedback":"ok"}`, cookie); w.Code != 403 {
		t.Fatal("student reviewed own answer")
	}
	if w = request(h, "PATCH", reviewURL, `{"status":"revision","feedback":"Добавьте таблицу"}`, teacher); w.Code != 200 {
		t.Fatal(w.Body)
	}
	exercises, _ := s.Exercises(studentUser.ID)
	found := false
	for _, exercise := range exercises {
		if exercise.ID == eid {
			found = exercise.Status == "revision"
		}
	}
	if !found {
		t.Fatal("revision not reflected in progress")
	}
	if w = request(h, "PATCH", reviewURL, `{"status":"passed","feedback":"Зачтено"}`, teacher); w.Code != 200 {
		t.Fatal(w.Body)
	}
	exercises, _ = s.Exercises(studentUser.ID)
	for _, exercise := range exercises {
		if exercise.ID == eid && exercise.Status != "passed" {
			t.Fatal("acceptance not reflected")
		}
	}
	w = request(h, "GET", fmt.Sprintf("/api/submissions/%d", job.ID), "", cookie)
	var result struct {
		Status string `json:"status"`
		Report struct {
			Feedback string `json:"feedback"`
		} `json:"report"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Status != "passed" || result.Report.Feedback != "Зачтено" {
		t.Fatal(w.Body)
	}
	if w = request(h, "POST", url, `{"code":"answer","mode":"run"}`, cookie); w.Code != 400 {
		t.Fatal("written work executed as Python")
	}
}

func TestShippedCatalogAcceptedByExerciseValidation(t *testing.T) {
	_, s := setup(t)
	exercises, err := s.Exercises(0)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, exercise := range exercises {
		if err = validateExercise(&exercise); err != nil {
			t.Fatalf("%s: %v", exercise.Title, err)
		}
		counts[exercise.Kind]++
	}
	if counts["written"] != 13 || counts["python"] != 45 || counts["python_review"] != 14 {
		t.Fatal(counts)
	}
}
