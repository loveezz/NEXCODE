"use client";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import {
  Download,
  FileJson,
  FolderPlus,
  Pencil,
  Plus,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import { api, send } from "@/lib/api";
import type { Catalog, Exercise, Lab } from "@/lib/types";
import { useUser } from "./shell";
import { Empty, ErrorState, Loading } from "./common";
import { Button } from "./ui/button";
type Editing =
  | { kind: "lab"; value: Partial<Lab> }
  | { kind: "exercise"; value: Partial<Exercise> };
export default function ManageView() {
  const { data: user } = useUser();
  const allowed = !!user && user.role !== "student";
  const client = useQueryClient();
  const fileInput = useRef<HTMLInputElement>(null);
  const [editing, setEditing] = useState<Editing | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [deleting, setDeleting] = useState<{
    kind: string;
    id: number;
    title: string;
  } | null>(null);
  const catalog = useQuery({
    queryKey: ["full-catalog"],
    queryFn: () => api<Catalog>("/teacher/catalog"),
    enabled: allowed,
  });
  const refresh = () => {
    client.invalidateQueries({ queryKey: ["full-catalog"] });
    client.invalidateQueries({ queryKey: ["catalog"] });
  };
  async function importFile(file: File) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      if (file.size > 2 * 1024 * 1024) throw new Error("Файл превышает 2 МБ");
      const body = JSON.parse(await file.text());
      const result = await send<{ labs: number; exercises: number }>(
        "/teacher/import",
        body,
      );
      setNotice(
        `Добавлено лабораторных: ${result.labs}, упражнений: ${result.exercises}.`,
      );
      refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
      if (fileInput.current) fileInput.current.value = "";
    }
  }
  async function save(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    let body: unknown;
    if (editing.kind === "lab")
      body = {
        title: f.get("title"),
        description: f.get("description"),
        position: Number(f.get("position")),
      };
    else
      body = {
        lab_id: Number(f.get("lab_id")),
        title: f.get("title"),
        statement: f.get("statement"),
        template: f.get("template"),
        tests: f.get("tests"),
        timeout_ms: Number(f.get("timeout_ms")),
        memory_mb: Number(f.get("memory_mb")),
        difficulty: f.get("difficulty"),
        position: Number(f.get("position")),
      };
    try {
      await send(
        `/teacher/${editing.kind === "lab" ? "labs" : "exercises"}${editing.value.id ? `/${editing.value.id}` : ""}`,
        body,
        editing.value.id ? "PUT" : "POST",
      );
      setEditing(null);
      setNotice("Изменения сохранены.");
      refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    if (!deleting) return;
    setBusy(true);
    setError("");
    try {
      await api(`/teacher/${deleting.kind}/${deleting.id}`, {
        method: "DELETE",
      });
      setDeleting(null);
      refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  function exportCatalog() {
    if (!catalog.data) return;
    const body = {
      labs: catalog.data.labs.map((l) => ({
        title: l.title,
        description: l.description,
        position: l.position,
        exercises: catalog.data.exercises
          .filter((e) => e.lab_id === l.id)
          .map((e) => ({
            title: e.title,
            statement: e.statement,
            template: e.template,
            tests: e.tests,
            timeout_ms: e.timeout_ms,
            memory_mb: e.memory_mb,
            difficulty: e.difficulty,
            position: e.position,
          })),
      })),
    };
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(body, null, 2)], { type: "application/json" }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = "nexcode-catalog.json";
    a.click();
    URL.revokeObjectURL(url);
  }
  if (user && !allowed)
    return (
      <ErrorState
        error={
          new Error(
            "Управление каталогом доступно преподавателям и администраторам.",
          )
        }
      />
    );
  return (
    <div>
      <div className="page-heading">
        <div>
          <div className="eyebrow">ВАШ УЧЕБНЫЙ МАТЕРИАЛ</div>
          <h1>Каталог заданий</h1>
          <p>Добавляйте лабораторные, условия и автотесты.</p>
        </div>
        <div className="heading-buttons">
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => fileInput.current?.click()}
          >
            <Upload size={15} />
            Импорт JSON
          </Button>
          <Button
            onClick={() => {
              setError("");
              setEditing({
                kind: "lab",
                value: { position: (catalog.data?.labs.length || 0) + 1 },
              });
            }}
          >
            <FolderPlus size={16} />
            Лабораторная
          </Button>
        </div>
        <input
          ref={fileInput}
          className="visually-hidden"
          type="file"
          accept=".json,application/json"
          aria-label="Импорт каталога"
          onChange={(e) => {
            if (e.target.files?.[0]) importFile(e.target.files[0]);
          }}
        />
      </div>
      <div className="import-hint">
        <FileJson size={21} />
        <div>
          <strong>Ваши лабораторные — ваши правила</strong>
          <p>
            Импортируйте JSON с условиями и тестами JSON Lines или создайте
            работу вручную. Демо-каталог можно использовать как образец.
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={exportCatalog}>
          <Download size={14} />
          Скачать каталог
        </Button>
      </div>
      {notice && (
        <div className="form-success" role="status">
          {notice}
        </div>
      )}
      {error && !editing && (
        <div className="form-error" role="alert">
          {error}
        </div>
      )}
      {catalog.isPending ? (
        <Loading />
      ) : catalog.error ? (
        <ErrorState error={catalog.error} retry={() => catalog.refetch()} />
      ) : !catalog.data?.labs.length ? (
        <Empty title="Добавьте первую лабораторную">
          Создайте работу вручную или импортируйте подготовленный JSON.
        </Empty>
      ) : (
        catalog.data.labs.map((l) => (
          <section className="manage-lab" key={l.id}>
            <div className="manage-lab-title">
              <div>
                <small>
                  ЛАБОРАТОРНАЯ {String(l.position).padStart(2, "0")}
                  {l.source === "demo" ? " · ДЕМО" : ""}
                </small>
                <h2>{l.title}</h2>
                <p>{l.description}</p>
              </div>
              <div>
                <button
                  className="icon-button"
                  aria-label={`Редактировать ${l.title}`}
                  onClick={() => {
                    setEditing({ kind: "lab", value: l });
                    setError("");
                  }}
                >
                  <Pencil size={15} />
                </button>
                <button
                  className="icon-button"
                  aria-label={`Удалить ${l.title}`}
                  onClick={() =>
                    setDeleting({ kind: "labs", id: l.id, title: l.title })
                  }
                >
                  <Trash2 size={15} />
                </button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => {
                    setError("");
                    setEditing({
                      kind: "exercise",
                      value: {
                        lab_id: l.id,
                        template: "# Ваше решение\n",
                        tests: '{"stdin":"2\\n3\\n","stdout":"5\\n"}',
                        timeout_ms: 5000,
                        memory_mb: 128,
                        position: 1,
                      },
                    });
                  }}
                >
                  <Plus size={14} />
                  Упражнение
                </Button>
              </div>
            </div>
            {catalog
              .data!.exercises.filter((e) => e.lab_id === l.id)
              .map((e) => (
                <div className="manage-exercise" key={e.id}>
                  <span className="mono muted">#{e.id}</span>
                  <strong>{e.title}</strong>
                  <small>
                    {e.test_count} теста · {e.timeout_ms} мс · {e.memory_mb} МБ
                  </small>
                  <button
                    className="icon-button"
                    aria-label={`Редактировать ${e.title}`}
                    onClick={() => {
                      setError("");
                      setEditing({ kind: "exercise", value: e });
                    }}
                  >
                    <Pencil size={15} />
                  </button>
                  <button
                    className="icon-button"
                    aria-label={`Удалить ${e.title}`}
                    onClick={() =>
                      setDeleting({
                        kind: "exercises",
                        id: e.id,
                        title: e.title,
                      })
                    }
                  >
                    <Trash2 size={15} />
                  </button>
                </div>
              ))}
          </section>
        ))
      )}
      {editing && (
        <div className="modal-backdrop">
          <section
            className="modal edit-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Редактирование каталога"
          >
            <div className="modal-title">
              <h2>
                {editing.value.id ? "Редактировать" : "Добавить"}{" "}
                {editing.kind === "lab" ? "лабораторную" : "упражнение"}
              </h2>
              <button
                className="icon-button"
                aria-label="Закрыть"
                onClick={() => {
                  setEditing(null);
                  setError("");
                }}
              >
                <X size={20} />
              </button>
            </div>
            <form onSubmit={save}>
              <label>
                Название
                <input
                  name="title"
                  defaultValue={editing.value.title}
                  minLength={2}
                  maxLength={200}
                  required
                />
              </label>
              {editing.kind === "lab" ? (
                <label>
                  Описание
                  <textarea
                    name="description"
                    defaultValue={editing.value.description}
                    maxLength={10000}
                    rows={3}
                  />
                </label>
              ) : (
                <>
                  <label>
                    Лабораторная
                    <select name="lab_id" defaultValue={editing.value.lab_id}>
                      {catalog.data?.labs.map((l) => (
                        <option value={l.id} key={l.id}>
                          {l.title}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Условие
                    <textarea
                      name="statement"
                      defaultValue={editing.value.statement}
                      maxLength={50000}
                      required
                      rows={5}
                    />
                  </label>
                  <label>
                    Шаблон кода
                    <textarea
                      className="mono"
                      name="template"
                      defaultValue={editing.value.template}
                      maxLength={65536}
                      rows={4}
                      spellCheck={false}
                    />
                  </label>
                  <label>
                    Тесты JSON Lines
                    <textarea
                      className="mono"
                      name="tests"
                      defaultValue={editing.value.tests}
                      required
                      maxLength={1048576}
                      rows={5}
                      spellCheck={false}
                    />
                    <small>
                      Один JSON на строку. Обязательные поля: stdin и stdout. До
                      20 тестов.
                    </small>
                  </label>
                  <div className="form-grid">
                    <label>
                      Таймаут, мс
                      <input
                        name="timeout_ms"
                        type="number"
                        min={100}
                        max={5000}
                        required
                        defaultValue={editing.value.timeout_ms}
                      />
                    </label>
                    <label>
                      Память, МБ
                      <input
                        name="memory_mb"
                        type="number"
                        min={64}
                        max={256}
                        required
                        defaultValue={editing.value.memory_mb}
                      />
                    </label>
                    <label>
                      Сложность
                      <select
                        name="difficulty"
                        defaultValue={editing.value.difficulty || "Базовый"}
                      >
                        <option>Базовый</option>
                        <option>Средний</option>
                        <option>Продвинутый</option>
                      </select>
                    </label>
                  </div>
                </>
              )}
              <label>
                Порядок в каталоге
                <input
                  name="position"
                  type="number"
                  min={0}
                  max={10000}
                  required
                  defaultValue={editing.value.position || 1}
                />
              </label>
              {error && (
                <div className="form-error" role="alert">
                  {error}
                </div>
              )}
              <div className="modal-actions">
                <Button
                  variant="secondary"
                  type="button"
                  onClick={() => {
                    setEditing(null);
                    setError("");
                  }}
                >
                  Отмена
                </Button>
                <Button type="submit" disabled={busy}>
                  {busy ? "Сохранение…" : "Сохранить"}
                </Button>
              </div>
            </form>
          </section>
        </div>
      )}
      {deleting && (
        <div className="modal-backdrop">
          <div
            className="modal compact-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Подтвердить удаление"
          >
            <h2>Удалить «{deleting.title}»?</h2>
            <p>
              Удаление доступно только для пустых лабораторных и упражнений без
              отправок. История решений сохраняется.
            </p>
            {error && (
              <div className="form-error" role="alert">
                {error}
              </div>
            )}
            <div className="modal-actions">
              <Button
                variant="secondary"
                onClick={() => {
                  setDeleting(null);
                  setError("");
                }}
              >
                Отмена
              </Button>
              <Button variant="danger" disabled={busy} onClick={remove}>
                Удалить
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
