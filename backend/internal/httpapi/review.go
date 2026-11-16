package httpapi

import (
	"encoding/json"
	"net/http"
	"nexcode/internal/store"
)

func (a *API) reviewSubmission(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status   string `json:"status"`
		Feedback string `json:"feedback"`
	}
	if !decode(w, r, &input) {
		return
	}
	if (input.Status != "passed" && input.Status != "revision") || len(input.Feedback) > 8000 {
		fail(w, 400, "Выберите зачёт или доработку; комментарий до 8000 байт")
		return
	}
	var kind, mode, raw string
	err := a.Store.DB.QueryRow(`SELECT e.kind,s.mode,s.report FROM submissions s JOIN exercises e ON e.id=s.exercise_id WHERE s.id=?`, id(r)).Scan(&kind, &mode, &raw)
	if err != nil {
		fail(w, 404, "Отправка не найдена")
		return
	}
	if kind == "python" || mode != "check" {
		fail(w, 400, "Для этой отправки используется автоматическая проверка")
		return
	}
	var report map[string]any
	if json.Unmarshal([]byte(raw), &report) != nil || report == nil {
		report = map[string]any{}
	}
	report["review"] = "revision"
	if input.Status == "passed" {
		report["review"] = "accepted"
	}
	report["feedback"] = input.Feedback
	report["reviewed_at"] = store.Now()
	report["reviewed_by"] = user(r).ID
	data, _ := json.Marshal(report)
	if _, err = a.Store.DB.Exec(`UPDATE submissions SET status=?,report=? WHERE id=?`, input.Status, string(data), id(r)); err != nil {
		fail(w, 500, "Не удалось сохранить проверку")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
