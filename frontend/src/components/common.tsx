"use client";
import { AlertCircle, Check, Circle, Loader2, RotateCw } from "lucide-react";
import { statusLabels } from "@/lib/api";
import { Button } from "./ui/button";
export function Status({ status }: { status: string }) {
  return (
    <span className={`status status-${status}`}>
      {status === "passed" ? (
        <Check size={12} />
      ) : status === "running" || status === "queued" ? (
        <Loader2 size={12} className="spin" />
      ) : (
        <Circle size={7} fill="currentColor" />
      )}
      {statusLabels[status] || status}
    </span>
  );
}
export function Loading({ text = "Загрузка…" }: { text?: string }) {
  return (
    <div className="loading">
      <Loader2 size={22} className="spin" />
      {text}
    </div>
  );
}
export function ErrorState({
  error,
  retry,
}: {
  error: unknown;
  retry?: () => void;
}) {
  return (
    <div className="error-state" role="alert">
      <AlertCircle size={22} />
      <div>
        <strong>Не удалось выполнить запрос</strong>
        <p>{error instanceof Error ? error.message : "Попробуйте ещё раз"}</p>
      </div>
      {retry && (
        <Button variant="secondary" onClick={retry}>
          <RotateCw size={14} />
          Повторить
        </Button>
      )}
    </div>
  );
}
export function Empty({
  title,
  children,
}: {
  title: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="empty">
      <div className="empty-mark">&lt;/&gt;</div>
      <h3>{title}</h3>
      {children && <p>{children}</p>}
    </div>
  );
}
export function Progress({ value }: { value: number }) {
  return (
    <div
      className="progress-track"
      role="progressbar"
      aria-valuenow={Math.round(value)}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div style={{ width: `${value}%` }} />
    </div>
  );
}
