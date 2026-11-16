"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { loader, type OnMount } from "@monaco-editor/react";
import type { editor, IDisposable } from "monaco-editor";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { AnimatePresence, motion } from "framer-motion";
import {
  ArrowLeft,
  Check,
  CheckCheck,
  ChevronRight,
  Clock3,
  Cloud,
  Code2,
  History,
  Loader2,
  MemoryStick,
  Play,
  RotateCcw,
  Terminal,
} from "lucide-react";
import { api, send } from "@/lib/api";
import type { Exercise, Submission } from "@/lib/types";
import { useEditorStore } from "@/lib/editor-store";
import { useTheme } from "./providers";
import { Empty, ErrorState, Loading, Status } from "./common";
import { Button } from "./ui/button";
import ReportView from "./report";
import DiagramPreview from "./diagram-preview";
import { basePath, isPreview } from "@/lib/runtime";
const Monaco = dynamic(() => import("@monaco-editor/react"), {
  ssr: false,
  loading: () => <Loading text="Загрузка редактора…" />,
});
loader.config({ paths: { vs: `${basePath}/monaco/vs` } });
export default function ExerciseEditor({ exerciseId }: { exerciseId: number }) {
  const client = useQueryClient();
  const theme = useTheme();
  const { code, setCode, initialize } = useEditorStore();
  const exercise = useQuery({
    queryKey: ["exercise", exerciseId],
    queryFn: () =>
      api<{
        exercise: Exercise;
        example: {
          stdin: string;
          stdout: string;
          files?: Record<string, string>;
        };
      }>(`/exercises/${exerciseId}`),
  });
  const draft = useQuery({
    queryKey: ["draft", exerciseId],
    queryFn: () =>
      api<{ code: string; exists: boolean }>(`/exercises/${exerciseId}/draft`),
    staleTime: 0,
  });
  const [ready, setReady] = useState(false);
  const [stdin, setStdin] = useState("");
  const [saving, setSaving] = useState("saved");
  const [draftError, setDraftError] = useState("");
  const [submitError, setSubmitError] = useState("");
  const [sending, setSending] = useState(false);
  const [jobId, setJobId] = useState<number | null>(null);
  const [confirmReset, setConfirmReset] = useState(false);
  const [tab, setTab] = useState<"result" | "input">("result");
  const lastSaved = useRef("");
  const saveChain = useRef<Promise<unknown>>(Promise.resolve());
  const initialized = useRef(false);
  const latestCode = useRef("");
  const latestReady = useRef(false);
  const action = useRef<(mode: string) => void>(() => {});
  const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null);
  const monacoRef = useRef<Parameters<OnMount>[1] | null>(null);
  const lastHandled = useRef<number | null>(null);
  const shortcut = useRef<IDisposable | null>(null);
  const job = useQuery({
    queryKey: ["submission", jobId],
    queryFn: () => api<Submission>(`/submissions/${jobId}`),
    enabled: jobId !== null,
    refetchInterval: (query) =>
      query.state.data &&
      !["queued", "running"].includes(query.state.data.status)
        ? false
        : 700,
  });
  const busy =
    sending ||
    (jobId !== null &&
      !job.isError &&
      (!job.data || ["queued", "running"].includes(job.data.status)));
  useEffect(() => {
    if (!initialized.current && exercise.data && draft.data) {
      const value = draft.data.exists
        ? draft.data.code
        : exercise.data.exercise.template;
      initialize(exerciseId, value);
      lastSaved.current = value;
      latestCode.current = value;
      setStdin(exercise.data.example.stdin);
      setReady(true);
      initialized.current = true;
    }
  }, [exercise.data, draft.data, exerciseId, initialize]);
  latestCode.current = code;
  latestReady.current = ready;
  const save = useCallback(
    (value: string) => {
      if (isPreview) return Promise.resolve();
      setSaving("saving");
      setDraftError("");
      const request = saveChain.current
        .catch(() => {})
        .then(() =>
          send(`/exercises/${exerciseId}/draft`, { code: value }, "PUT"),
        );
      saveChain.current = request;
      request
        .then(() => {
          lastSaved.current = value;
          if (latestCode.current === value) setSaving("saved");
          client.invalidateQueries({ queryKey: ["catalog"] });
        })
        .catch((e) => {
          setSaving("error");
          setDraftError(e.message);
        });
      return request;
    },
    [exerciseId, client],
  );
  useEffect(() => {
    if (isPreview || !ready || code === lastSaved.current) return;
    setSaving("waiting");
    const timer = setTimeout(() => {
      save(code);
    }, 2000);
    return () => clearTimeout(timer);
  }, [code, ready, save]);
  useEffect(
    () => () => {
      shortcut.current?.dispose();
      if (
        !isPreview &&
        latestReady.current &&
        latestCode.current !== lastSaved.current
      ) {
        const value = latestCode.current;
        saveChain.current
          .catch(() => {})
          .then(() =>
            api(`/exercises/${exerciseId}/draft`, {
              method: "PUT",
              body: JSON.stringify({ code: value }),
              keepalive: true,
            }),
          )
          .catch(() => {});
      }
    },
    [exerciseId],
  );
  useEffect(() => {
    if (
      !job.data ||
      ["queued", "running"].includes(job.data.status) ||
      lastHandled.current === job.data.id
    )
      return;
    lastHandled.current = job.data.id;
    client.invalidateQueries({ queryKey: ["catalog"] });
    client.invalidateQueries({ queryKey: ["history"] });
  }, [job.data, client]);
  useEffect(() => {
    const model = editorRef.current?.getModel();
    const monaco = monacoRef.current;
    if (model && monaco)
      monaco.editor.setModelMarkers(
        model,
        "ruff",
        (job.data?.report.lint || []).map((l) => ({
          startLineNumber: l.location.row,
          startColumn: l.location.column,
          endLineNumber: l.location.row,
          endColumn: l.location.column + 1,
          message: `${l.code}: ${l.message}`,
          severity: monaco.MarkerSeverity.Warning,
        })),
      );
  }, [job.data]);
  async function execute(mode: string) {
    if (isPreview || busy || !ready) return;
    if (exercise.data?.exercise.kind === "written") mode = "check";
    setSending(true);
    setSubmitError("");
    setTab("result");
    try {
      const out = await send<{ id: number }>(
        `/exercises/${exerciseId}/submit`,
        { code: useEditorStore.getState().code, stdin, mode },
      );
      setJobId(out.id);
    } catch (e) {
      setSubmitError((e as Error).message);
    } finally {
      setSending(false);
    }
  }
  action.current = (mode) => {
    void execute(mode);
  };
  const mount: OnMount = (ed, monaco) => {
    editorRef.current = ed;
    monacoRef.current = monaco;
    ed.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () =>
      action.current("run"),
    );
    shortcut.current = ed.onDidDispose(() => {
      editorRef.current = null;
    });
  };
  if (exercise.isPending || draft.isPending)
    return <Loading text="Восстанавливаем рабочее пространство…" />;
  if (exercise.error || draft.error)
    return (
      <ErrorState
        error={exercise.error || draft.error}
        retry={() => {
          exercise.refetch();
          draft.refetch();
        }}
      />
    );
  if (!exercise.data || !ready) return <Loading />;
  const e = exercise.data.exercise;
  const written = e.kind === "written";
  const manual = e.kind !== "python";
  const inputFiles = exercise.data.example.files || {};
  return (
    <div className="exercise-workspace">
      <div className="editor-heading">
        <div>
          <Link href="/labs">
            <ArrowLeft size={15} />К лабораторным
          </Link>
          <span>/</span>
          <strong>{e.title}</strong>
        </div>
        <Link href={`/history?exercise=${e.id}`}>
          <History size={15} />
          История решений
          <ChevronRight size={14} />
        </Link>
      </div>
      <div className="editor-grid">
        <section className="statement-panel">
          <div className="panel-title">
            <BookIcon />
            Условие задачи<span>#{String(e.id).padStart(3, "0")}</span>
          </div>
          <div className="statement-body">
            <span className="eyebrow">
              {written
                ? "ПИСЬМЕННАЯ РАБОТА"
                : "PYTHON · " + e.difficulty.toUpperCase()}
            </span>
            <h1>{e.title}</h1>
            <p className="statement-text">{e.statement}</p>
            {manual && (
              <p className="manual-note">
                Работу проверяет преподаватель. После сдачи результат появится в
                истории.
              </p>
            )}
            {!written && (
              <div className="limits">
                <span>
                  <Clock3 size={14} />
                  {e.timeout_ms / 1000} с
                </span>
                <span>
                  <MemoryStick size={14} />
                  {e.memory_mb} МБ
                </span>
                <span>
                  <CheckCheck size={14} />
                  {manual
                    ? "Проверка преподавателем"
                    : e.test_count + " тестов"}
                </span>
              </div>
            )}
            {!written && (
              <>
                <h3>Пример</h3>
                <div className="example">
                  <label>
                    Ввод<pre>{exercise.data.example.stdin}</pre>
                  </label>
                  <label>
                    Вывод<pre>{exercise.data.example.stdout}</pre>
                  </label>
                </div>
                {Object.entries(inputFiles).map(([name, content]) => (
                  <details className="virtual-file" key={name} open>
                    <summary>{name}</summary>
                    <pre>{content || "(пустой файл)"}</pre>
                  </details>
                ))}
                <div className="statement-note">
                  <Code2 size={18} />
                  <p>
                    {manual
                      ? "Выбирайте данные и оформление по условию; используйте запуск для самопроверки."
                      : "Соблюдайте указанный формат ввода и вывода. Для файловых задач используйте учебные файлы."}
                  </p>
                </div>
                <details className="sandbox-note">
                  <summary>Об окружении</summary>
                  <p>
                    Python 3.12. Доступны math, random, re, statistics,
                    collections, itertools, functools, string, decimal,
                    fractions, datetime, json, heapq, bisect, operator, array.
                    Доступны учебные файлы через open(), os.path.exists() и
                    pathlib.Path. Файлы сервера, сеть и запуск процессов
                    недоступны.
                  </p>
                </details>
              </>
            )}
          </div>
        </section>
        <section className="code-panel">
          <div className="code-panel-toolbar">
            <span>
              <span className="python-dot" />
              {written ? "Ответ.md" : "main.py"}
            </span>
            <div>
              <small className={saving === "error" ? "save-error" : ""}>
                {saving === "saved" ? <Check size={12} /> : <Cloud size={12} />}
                {isPreview
                  ? "Просмотр без сохранения"
                  : saving === "saved"
                    ? "Черновик сохранён"
                    : saving === "error"
                      ? "Не сохранён"
                      : "Сохранение…"}
              </small>
              <button
                aria-label="Сбросить код"
                className="icon-button"
                onClick={() => setConfirmReset(true)}
              >
                <RotateCcw size={15} />
              </button>
            </div>
          </div>
          <div className="monaco-wrap">
            <Monaco
              height="100%"
              language={written ? "markdown" : "python"}
              path={`exercise-${exerciseId}.py`}
              value={code}
              onChange={(v) => setCode(v || "")}
              theme={theme.dark ? "vs-dark" : "vs"}
              onMount={mount}
              options={{
                fontSize: 16,
                fontFamily: "Consolas, monospace",
                lineHeight: 28,
                minimap: { enabled: false },
                padding: { top: 22, bottom: 20 },
                scrollBeyondLastLine: false,
                automaticLayout: true,
                tabSize: 4,
                insertSpaces: true,
                wordWrap: "on",
                renderLineHighlight: "gutter",
                overviewRulerLanes: 0,
                hideCursorInOverviewRuler: true,
              }}
            />
          </div>
          {draftError && (
            <div className="draft-error" role="alert">
              {draftError}
              <button onClick={() => save(code)}>Повторить сохранение</button>
            </div>
          )}
          <div className="run-toolbar">
            <span>
              {written ? "Письменный ответ" : "Python 3.12"}
              <small>Ctrl + Enter — {written ? "сдать" : "запуск"}</small>
            </span>
            <div>
              {!written && (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={busy || isPreview}
                  onClick={() => execute("run")}
                >
                  <Play size={14} />
                  Запустить
                </Button>
              )}
              <Button
                size="sm"
                disabled={busy || isPreview}
                onClick={() => execute("check")}
              >
                {busy ? (
                  <Loader2 size={15} className="spin" />
                ) : (
                  <CheckCheck size={15} />
                )}
                {manual ? "Сдать преподавателю" : "Проверить"}
              </Button>
            </div>
          </div>
          <div className="console-panel">
            <div className="console-tabs">
              <button
                className={tab === "result" ? "selected" : ""}
                onClick={() => setTab("result")}
              >
                <Terminal size={14} />
                Результаты
                {job.data && !manual && (
                  <span>
                    {job.data.report.passed ?? 0}/
                    {job.data.report.total ?? e.test_count}
                  </span>
                )}
              </button>
              {!written && (
                <button
                  className={tab === "input" ? "selected" : ""}
                  onClick={() => setTab("input")}
                >
                  Входные данные
                </button>
              )}
              <span>{manual ? "Проверка преподавателем" : "Автотесты"}</span>
            </div>
            <div className="console-body" aria-live="polite">
              {tab === "input" ? (
                <label className="stdin-label">
                  Данные для кнопки «Запустить». Проверка использует тесты
                  упражнения.
                  <textarea
                    aria-label="Стандартный ввод"
                    value={stdin}
                    onChange={(e) => setStdin(e.target.value)}
                    spellCheck={false}
                    maxLength={32768}
                  />
                </label>
              ) : (
                <>
                  {submitError && (
                    <div className="form-error" role="alert">
                      {submitError}
                    </div>
                  )}
                  {job.error && (
                    <ErrorState error={job.error} retry={() => job.refetch()} />
                  )}
                  {busy ? (
                    <div className="checking">
                      <Loader2 size={24} className="spin" />
                      <strong>
                        {job.data?.status === "running"
                          ? "Исполняем код в песочнице"
                          : "Ожидаем свободный исполнитель"}
                      </strong>
                      <p>Результат появится автоматически</p>
                    </div>
                  ) : job.data ? (
                    <ReportView report={job.data.report} mode={job.data.mode} />
                  ) : (
                    <Empty
                      title={
                        isPreview
                          ? "Редактор доступен для просмотра"
                          : "Проверим вашу идею?"
                      }
                    >
                      <span>
                        {isPreview
                          ? "Запуск Python и сохранение решений появятся после подключения сервера."
                          : written
                            ? "Запишите ответ и сдайте его преподавателю."
                            : "Запустите программу или отправьте решение на проверку."}
                      </span>
                    </Empty>
                  )}
                </>
              )}
              {written && <DiagramPreview text={code} />}
            </div>
          </div>
        </section>
      </div>
      <AnimatePresence>
        {confirmReset && (
          <motion.div
            className="modal-backdrop"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            <div
              role="dialog"
              aria-modal="true"
              aria-labelledby="reset-title"
              className="modal compact-modal"
            >
              <h2 id="reset-title">Вернуть исходный шаблон?</h2>
              <p>
                Текущий черновик будет заменён. История отправок сохранится.
              </p>
              <div className="modal-actions">
                <Button
                  variant="secondary"
                  onClick={() => setConfirmReset(false)}
                >
                  Отмена
                </Button>
                <Button
                  onClick={() => {
                    setCode(e.template);
                    setConfirmReset(false);
                  }}
                >
                  Сбросить код
                </Button>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
function BookIcon() {
  return <Code2 size={16} />;
}
