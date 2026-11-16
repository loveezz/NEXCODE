"use client";
import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { exerciseHref } from "@/lib/runtime";
import {
  ArrowUpRight,
  ChevronRight,
  Code2,
  History as HistoryIcon,
  X,
} from "lucide-react";
import { api, date, send } from "@/lib/api";
import type { Submission } from "@/lib/types";
import { Empty, ErrorState, Loading, Status } from "./common";
import ReportView from "./report";
import DiagramPreview from "./diagram-preview";
import { useUser } from "./shell";
import { Button } from "./ui/button";
export default function HistoryView() {
  const { data: currentUser } = useUser();
  const canReview =
    currentUser?.role === "teacher" || currentUser?.role === "admin";
  const client = useQueryClient();
  const [feedback, setFeedback] = useState("");
  const [reviewError, setReviewError] = useState("");
  const [reviewing, setReviewing] = useState(false);
  const params = useSearchParams();
  const [selected, setSelected] = useState<number | null>(
    params.get("id") ? Number(params.get("id")) : null,
  );
  const [filter, setFilter] = useState("all");
  useEffect(() => {
    setFeedback("");
    setReviewError("");
  }, [selected]);
  const all = params.get("all") === "1";
  const student = params.get("student");
  const exercise = params.get("exercise");
  const history = useQuery({
    queryKey: ["history", all, exercise],
    queryFn: () =>
      api<Submission[]>(
        `/submissions?all=${all ? "1" : "0"}&exercise_id=${exercise || ""}`,
      ),
  });
  const detail = useQuery({
    queryKey: ["submission", selected],
    queryFn: () => api<Submission>(`/submissions/${selected}`),
    enabled: selected !== null,
    refetchInterval: (q) =>
      q.state.data && ["running", "queued"].includes(q.state.data.status)
        ? 1000
        : false,
  });
  const visible = history.data?.filter(
    (s) =>
      (!student || s.user_id === Number(student)) &&
      (filter === "all" ||
        (filter === "review"
          ? s.status === "review"
          : filter === "check"
            ? s.mode === "check"
            : s.mode === "run")),
  );
  return (
    <div>
      <div className="page-heading">
        <div>
          <div className="eyebrow">КАЖДАЯ ПОПЫТКА ИМЕЕТ ЗНАЧЕНИЕ</div>
          <h1>История отправок</h1>
          <p>Решения, результаты тестов и замечания — в одном месте.</p>
        </div>
        <span className="semester">
          <HistoryIcon size={15} />
          {history.data?.length || 0} отправок
        </span>
      </div>
      <div className="history-filters">
        {[
          ["all", "Все отправки"],
          ["check", "Проверки"],
          ["review", "Ждут преподавателя"],
          ["run", "Запуски"],
        ].map(([v, label]) => (
          <button
            key={v}
            className={filter === v ? "selected" : ""}
            onClick={() => setFilter(v)}
          >
            {label}
          </button>
        ))}
      </div>
      {history.isPending ? (
        <Loading />
      ) : history.error ? (
        <ErrorState error={history.error} retry={() => history.refetch()} />
      ) : visible?.length ? (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Отправка / упражнение</th>
                {all && <th>Студент</th>}
                <th>Режим</th>
                <th>Результат</th>
                <th>Отправлено</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {visible.map((s) => (
                <tr key={s.id} onClick={() => setSelected(s.id)}>
                  <td>
                    <button
                      className="table-link"
                      onClick={() => setSelected(s.id)}
                    >
                      <span className="mono muted">#{s.id}</span>
                      <strong>{s.title}</strong>
                    </button>
                  </td>
                  {all && <td>{s.login}</td>}
                  <td>
                    <span className="mode-tag">
                      {s.mode === "check" ? "Проверка" : "Запуск"}
                    </span>
                  </td>
                  <td>
                    <Status status={s.status} />
                  </td>
                  <td className="muted">{date(s.created_at)}</td>
                  <td>
                    <ChevronRight size={15} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <Empty title="Пока нет отправок">
          Откройте упражнение и отправьте первое решение.
          <br />
          <Link className="text-link" href="/labs">
            К лабораторным
            <ArrowUpRight size={14} />
          </Link>
        </Empty>
      )}
      {selected !== null && (
        <div className="modal-backdrop" onClick={() => setSelected(null)}>
          <section
            className="modal history-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Подробности отправки"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="modal-title">
              <div>
                <small>ОТПРАВКА #{selected}</small>
                <h2>{detail.data?.title || "Загрузка…"}</h2>
              </div>
              <button
                className="icon-button"
                onClick={() => setSelected(null)}
                aria-label="Закрыть"
              >
                <X size={21} />
              </button>
            </div>
            {detail.isPending ? (
              <Loading />
            ) : detail.error ? (
              <ErrorState error={detail.error} retry={() => detail.refetch()} />
            ) : (
              detail.data && (
                <>
                  <div className="submission-meta">
                    <Status status={detail.data.status} />
                    <span>{date(detail.data.created_at)}</span>
                    <Button asChild size="sm" variant="secondary">
                      <Link href={exerciseHref(detail.data.exercise_id)}>
                        <Code2 size={14} />
                        Открыть упражнение
                      </Link>
                    </Button>
                  </div>
                  <h4>Решение</h4>
                  <pre className="source-code">{detail.data.code}</pre>
                  {detail.data.report.review && (
                    <DiagramPreview text={detail.data.code} />
                  )}
                  {canReview && detail.data.report.review && (
                    <div className="review-form">
                      <label>
                        Комментарий преподавателя
                        <textarea
                          aria-label="Комментарий преподавателя"
                          value={feedback}
                          onChange={(event) => setFeedback(event.target.value)}
                          maxLength={4000}
                          rows={3}
                        />
                      </label>
                      {reviewError && (
                        <p role="alert" className="form-error">
                          {reviewError}
                        </p>
                      )}
                      <div>
                        {[
                          ["passed", "Зачесть"],
                          ["revision", "На доработку"],
                        ].map(([status, label]) => (
                          <Button
                            key={status}
                            disabled={reviewing}
                            variant={
                              status === "passed" ? "default" : "secondary"
                            }
                            onClick={async () => {
                              setReviewing(true);
                              setReviewError("");
                              try {
                                await send(
                                  "/teacher/submissions/" +
                                    selected +
                                    "/review",
                                  { status, feedback },
                                  "PATCH",
                                );
                                await Promise.all(
                                  [
                                    "submission",
                                    "history",
                                    "catalog",
                                    "students",
                                  ].map((key) =>
                                    client.invalidateQueries({
                                      queryKey: [key],
                                    }),
                                  ),
                                );
                              } catch (error) {
                                setReviewError((error as Error).message);
                              } finally {
                                setReviewing(false);
                              }
                            }}
                          >
                            {label}
                          </Button>
                        ))}
                      </div>
                    </div>
                  )}
                  <ReportView
                    report={detail.data.report}
                    mode={detail.data.mode}
                  />
                </>
              )
            )}
          </section>
        </div>
      )}
    </div>
  );
}
