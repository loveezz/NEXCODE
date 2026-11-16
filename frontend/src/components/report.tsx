"use client";
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  Clock3,
  XCircle,
} from "lucide-react";
import type { Report } from "@/lib/types";
export default function ReportView({
  report,
  mode = "check",
}: {
  report: Report;
  mode?: string;
}) {
  return (
    <div className="report-view">
      {report.review && (
        <div className={"manual-review manual-review-" + report.review}>
          <strong>
            {report.review === "pending"
              ? "На проверке преподавателя"
              : report.review === "accepted"
                ? "Работа зачтена"
                : "Нужна доработка"}
          </strong>
          {report.feedback && <p>{report.feedback}</p>}
        </div>
      )}
      {report.error && (
        <div className="form-error" role="alert">
          {report.error}
        </div>
      )}
      {report.results && report.results.length > 0 && (
        <>
          <div
            className={`report-summary ${report.passed === report.total ? "success" : "failure"}`}
          >
            {report.passed === report.total ? (
              <CheckCircle2 size={19} />
            ) : (
              <XCircle size={19} />
            )}
            <strong>
              {mode === "run"
                ? report.passed === report.total
                  ? "Программа выполнена"
                  : "Ошибка программы"
                : `Пройдено ${report.passed} из ${report.total} тестов`}
            </strong>
            <span>
              {report.results
                .reduce((sum, r) => sum + r.duration_ms, 0)
                .toFixed(0)}{" "}
              мс
            </span>
          </div>
          {report.results.map((r, i) => (
            <details
              className={`test-result ${r.passed ? "test-pass" : "test-fail"}`}
              key={i}
              open={!r.passed || mode === "run"}
            >
              <summary>
                {r.passed ? <CheckCircle2 size={15} /> : <XCircle size={15} />}
                <strong>
                  {mode === "run" ? "Вывод программы" : `Тест ${i + 1}`}
                </strong>
                <span>
                  <Clock3 size={12} />
                  {r.duration_ms.toFixed(0)} мс
                </span>
                <ChevronDown size={14} />
              </summary>
              <div className="test-detail">
                {r.timed_out && (
                  <div className="form-error">Превышен лимит времени</div>
                )}
                {r.output_limit && (
                  <div className="form-error">Превышен лимит вывода</div>
                )}
                <label>
                  Входные данные<pre>{r.stdin || "∅"}</pre>
                </label>
                {mode === "check" && (
                  <label>
                    Ожидаемый вывод<pre>{r.expected || "∅"}</pre>
                  </label>
                )}
                <label>
                  {mode === "run" ? "stdout" : "Фактический вывод"}
                  <pre>{r.actual || "(нет вывода)"}</pre>
                </label>
                {r.stderr && (
                  <label className="stderr">
                    stderr<pre>{r.stderr}</pre>
                  </label>
                )}
                {Object.entries(r.files || {}).map(([name, content]) => (
                  <label key={name}>
                    Файл {name}
                    <pre>{content || "(пустой файл)"}</pre>
                    {r.expected_files?.[name] !== undefined && (
                      <>
                        Ожидаемое содержимое
                        <pre>{r.expected_files[name] || "(пустой файл)"}</pre>
                      </>
                    )}
                  </label>
                ))}
                {Object.keys(r.expected_files || {})
                  .filter((name) => !(name in (r.files || {})))
                  .map((name) => (
                    <label className="stderr" key={name}>
                      Файл {name} не создан<pre>{r.expected_files![name]}</pre>
                    </label>
                  ))}
              </div>
            </details>
          ))}
        </>
      )}
      {!!report.lint?.length && (
        <div className="lint-list">
          <h4>
            <AlertTriangle size={15} />
            Ruff · {report.lint.length} замечаний
          </h4>
          {report.lint.map((l, i) => (
            <div key={i}>
              <code>{l.code}</code>
              <span>
                Строка {l.location.row}:{l.location.column}
              </span>
              <p>{l.message}</p>
            </div>
          ))}
        </div>
      )}
      {report.results?.length && report.lint?.length === 0 ? (
        <div className="lint-clean">
          <CheckCircle2 size={14} />
          Ruff: замечаний нет
        </div>
      ) : null}
    </div>
  );
}
