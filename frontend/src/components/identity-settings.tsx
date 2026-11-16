"use client";
import { useState, useRef } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  CalendarDays,
  Check,
  Download,
  KeyRound,
  LogOut,
  Plus,
  ShieldCheck,
  Upload,
  X,
} from "lucide-react";
import { api, send, date } from "@/lib/api";
import type { LoginEvent, LoginOptions, StudentGroup } from "@/lib/types";
import { useUser } from "./shell";
import { Button } from "./ui/button";
import { Empty, ErrorState, Loading } from "./common";
type Invite = { id?: number; full_name: string; code: string };
export default function IdentitySettings() {
  const { data: user } = useUser();
  const allowed = !!user && user.role !== "student";
  const client = useQueryClient();
  const params = useSearchParams();
  const [tab, setTab] = useState(
    params.get("student") ? "journal" : "settings",
  );
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState<
    StudentGroup | { id: 0; name: string; invite_required: boolean } | null
  >(null);
  const [roster, setRoster] = useState<StudentGroup | null>(null);
  const [text, setText] = useState("");
  const [filter, setFilter] = useState("");
  const [before, setBefore] = useState<number | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const [clear, setClear] = useState<{
    group?: StudentGroup;
    student?: string;
  } | null>(null);
  const settings = useQuery({
    queryKey: ["identity-settings"],
    queryFn: () => api<LoginOptions>("/teacher/identity"),
    enabled: allowed,
  });
  const invites = useQuery({
    queryKey: ["invites", roster?.id],
    queryFn: () => api<Invite[]>(`/teacher/groups/${roster!.id}/invites`),
    enabled: !!roster,
  });
  const events = useQuery({
    queryKey: ["login-events", filter, params.get("student"), before],
    queryFn: () =>
      api<{ events: LoginEvent[]; next_before: number }>(
        `/teacher/login-events?group=${encodeURIComponent(filter)}&student_id=${params.get("student") || ""}&before=${before || ""}`,
      ),
    enabled: allowed && tab === "journal",
    refetchInterval: before ? false : 15000,
  });
  const refresh = () => {
    client.invalidateQueries({ queryKey: ["identity-settings"] });
    client.invalidateQueries({ queryKey: ["login-options"] });
    client.invalidateQueries({ queryKey: ["students"] });
  };
  async function perform(action: () => Promise<unknown>, message: string) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await action();
      setNotice(message);
      refresh();
      return true;
    } catch (e) {
      setError((e as Error).message);
      return false;
    } finally {
      setBusy(false);
    }
  }
  async function saveSettings(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    await perform(
      () =>
        send(
          "/teacher/identity",
          {
            group_mode: f.get("group_mode"),
            semester_end: f.get("semester_end"),
          },
          "PUT",
        ),
      "Настройки сохранены. Срок активных ученических сессий обновлён.",
    );
  }
  async function saveGroup(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    const f = new FormData(e.currentTarget);
    if (
      await perform(
        () =>
          send(
            `/teacher/groups${editing.id ? `/${editing.id}` : ""}`,
            {
              name: f.get("name"),
              invite_required: f.get("invite_required") === "on",
            },
            editing.id ? "PUT" : "POST",
          ),
        "Группа сохранена.",
      )
    )
      setEditing(null);
  }
  async function importRoster() {
    if (!roster) return;
    try {
      let students: Invite[];
      if (text.trim().startsWith("{") || text.trim().startsWith("[")) {
        const data = JSON.parse(text);
        students = Array.isArray(data) ? data : data.students;
      } else
        students = text
          .split(/\r?\n/)
          .filter((v) => v.trim())
          .map((line) => {
            const [full_name, code, ...extra] = line.split(";");
            if (extra.length || !code)
              throw new Error(
                "Используйте формат «ФИО;КОД», по одному студенту на строку.",
              );
            return { full_name: full_name.trim(), code: code.trim() };
          });
      if (
        await perform(
          () => send(`/teacher/groups/${roster.id}/invites`, { students }),
          "Список кодов загружен.",
        )
      ) {
        setText("");
        client.invalidateQueries({ queryKey: ["invites", roster.id] });
      }
    } catch (e) {
      setError((e as Error).message);
    }
  }
  function downloadCodes() {
    const rows = [
      "ФИО;Код",
      ...(invites.data || []).map((v) => `${v.full_name};${v.code}`),
    ];
    const url = URL.createObjectURL(
      new Blob(["\uFEFF" + rows.join("\r\n")], {
        type: "text/csv;charset=utf-8",
      }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = `${roster?.name}-codes.csv`;
    link.click();
    URL.revokeObjectURL(url);
  }
  async function clearSessions() {
    if (!clear) return;
    const endpoint = clear.group
      ? `/teacher/groups/${clear.group.id}/sessions`
      : `/teacher/students/${clear.student}/sessions`;
    if (
      await perform(
        () => api(endpoint, { method: "DELETE" }),
        "Ученические сессии завершены. Прогресс сохранён.",
      )
    )
      setClear(null);
  }
  if (user && !allowed)
    return (
      <ErrorState
        error={new Error("Раздел доступен преподавателям и администраторам.")}
      />
    );
  return (
    <div>
      <div className="page-heading">
        <div>
          <div className="eyebrow">УЧЕНИЧЕСКИЙ ВХОД</div>
          <h1>Группы и вход</h1>
          <p>Срок семестра, индивидуальные коды и история посещений.</p>
        </div>
        <span className="semester">
          <ShieldCheck size={15} />
          Управление доступом
        </span>
      </div>
      <div className="history-filters">
        {[
          ["settings", "Настройки и группы"],
          ["journal", "Журнал входов"],
        ].map(([v, label]) => (
          <button
            className={tab === v ? "selected" : ""}
            key={v}
            onClick={() => {
              setTab(v);
              setError("");
            }}
          >
            {label}
          </button>
        ))}
      </div>
      {error && !editing && !roster && !clear && (
        <div className="form-error" role="alert">
          {error}
        </div>
      )}
      {notice && (
        <div className="form-success" role="status">
          <Check size={16} />
          {notice}
        </div>
      )}
      {settings.isPending ? (
        <Loading />
      ) : settings.error ? (
        <ErrorState error={settings.error} retry={() => settings.refetch()} />
      ) : tab === "settings" ? (
        <>
          <section className="identity-card">
            <div className="identity-card-heading">
              <CalendarDays size={21} />
              <div>
                <h2>Срок доступа студентов</h2>
                <p>
                  Выберите последний день семестра. Настройка применяется ко
                  всем активным ученическим сессиям.
                </p>
              </div>
            </div>
            <form
              key={`${settings.data?.semester_end}-${settings.data?.group_mode}`}
              onSubmit={saveSettings}
              className="identity-form"
            >
              <label>
                Дата окончания семестра
                <input
                  type="date"
                  name="semester_end"
                  defaultValue={settings.data?.semester_end}
                  required
                />
              </label>
              <label>
                Поле «Группа»
                <select
                  name="group_mode"
                  defaultValue={settings.data?.group_mode}
                >
                  <option value="free">Свободный ввод и подсказки</option>
                  <option value="list">Выбор из списка преподавателя</option>
                </select>
              </label>
              <Button type="submit" disabled={busy}>
                Сохранить настройки
              </Button>
            </form>
            <p className="identity-hint">
              Доступ действует до конца выбранного дня по времени сервера. На
              используемых браузерах срок cookie обновится при следующем
              запросе.
            </p>
          </section>
          <div className="section-heading">
            <h2>
              Справочник групп<span>{settings.data?.groups.length}</span>
            </h2>
            <Button
              size="sm"
              onClick={() => {
                setError("");
                setEditing({ id: 0, name: "", invite_required: false });
              }}
            >
              <Plus size={14} />
              Добавить группу
            </Button>
          </div>
          {!settings.data?.groups.length ? (
            <Empty title="Добавьте первую группу">
              Или разрешите свободный ввод — группа появится после входа
              студента.
            </Empty>
          ) : (
            <div className="identity-groups">
              {settings.data.groups.map((g) => (
                <section className="identity-card" key={g.id}>
                  <div className="identity-group-title">
                    <span className="lab-symbol">
                      <KeyRound size={20} />
                    </span>
                    <div>
                      <h3>{g.name}</h3>
                      <p>
                        {g.members || 0} студентов · {g.codes || 0} кодов
                      </p>
                    </div>
                    <span
                      className={`status ${g.invite_required ? "status-passed" : ""}`}
                    >
                      {g.invite_required ? "Вход с кодом" : "ФИО + группа"}
                    </span>
                  </div>
                  <div className="identity-group-actions">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => {
                        setError("");
                        setEditing(g);
                      }}
                    >
                      Настроить
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => {
                        setError("");
                        setText("");
                        setRoster(g);
                      }}
                    >
                      Список и коды
                    </Button>
                    <button
                      className="icon-button"
                      title="Завершить сессии группы"
                      aria-label={`Завершить сессии ${g.name}`}
                      onClick={() => {
                        setError("");
                        setClear({ group: g });
                      }}
                    >
                      <LogOut size={16} />
                    </button>
                  </div>
                </section>
              ))}
            </div>
          )}
        </>
      ) : (
        <>
          <div className="catalog-toolbar">
            <select
              aria-label="Группа в журнале"
              value={filter}
              onChange={(e) => {
                setFilter(e.target.value);
                setBefore(null);
              }}
            >
              <option value="">Все группы</option>
              {settings.data?.groups.map((g) => (
                <option key={g.id} value={g.name}>
                  {g.name}
                </option>
              ))}
            </select>
            {params.get("student") && (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setClear({ student: params.get("student")! })}
              >
                <LogOut size={14} />
                Завершить сессии студента
              </Button>
            )}
            <span>Успехи и отказы · последние 100 записей</span>
          </div>
          {events.isPending ? (
            <Loading />
          ) : events.error ? (
            <ErrorState error={events.error} retry={() => events.refetch()} />
          ) : !events.data?.events.length ? (
            <Empty title="Записей пока нет">
              Здесь появятся время, ФИО, группа, IP и результат входа.
            </Empty>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Время</th>
                    <th>ФИО</th>
                    <th>Группа</th>
                    <th>IP</th>
                    <th>Результат</th>
                  </tr>
                </thead>
                <tbody>
                  {events.data.events.map((e) => (
                    <tr key={e.id}>
                      <td className="muted">{date(e.created_at)}</td>
                      <td>{e.full_name || "—"}</td>
                      <td>{e.group_name || "—"}</td>
                      <td className="mono muted">{e.ip}</td>
                      <td>
                        <span
                          className={`status ${e.success ? "status-passed" : "status-failed"}`}
                        >
                          {e.success ? "Успех" : "Отказ"}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <div className="journal-pagination">
            {before && (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setBefore(null)}
              >
                К последним записям
              </Button>
            )}
            {!!events.data?.next_before && (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setBefore(events.data!.next_before)}
              >
                Предыдущие записи
              </Button>
            )}
          </div>
        </>
      )}
      {editing && (
        <div className="modal-backdrop">
          <section
            className="modal compact-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Настройка группы"
          >
            <div className="modal-title">
              <h2>{editing.id ? "Настройка группы" : "Новая группа"}</h2>
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
            <form onSubmit={saveGroup}>
              <label>
                Название группы
                <input
                  name="name"
                  defaultValue={editing.name}
                  required
                  maxLength={20}
                />
              </label>
              <label className="checkbox-label">
                <input
                  type="checkbox"
                  name="invite_required"
                  defaultChecked={editing.invite_required}
                />
                Требовать индивидуальный код
              </label>
              <p className="identity-hint">
                При изменении режима текущие сессии группы будут завершены. Для
                входа по кодам загрузите список студентов.
              </p>
              {error && (
                <div className="form-error" role="alert">
                  {error}
                </div>
              )}
              <div className="modal-actions">
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    setEditing(null);
                    setError("");
                  }}
                >
                  Отмена
                </Button>
                <Button disabled={busy} type="submit">
                  Сохранить группу
                </Button>
              </div>
            </form>
          </section>
        </div>
      )}
      {roster && (
        <div className="modal-backdrop">
          <section
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-label="Список приглашений"
          >
            <div className="modal-title">
              <div>
                <small>ГРУППА {roster.name}</small>
                <h2>Студенты и коды приглашений</h2>
              </div>
              <button
                className="icon-button"
                aria-label="Закрыть"
                onClick={() => {
                  setRoster(null);
                  setError("");
                }}
              >
                <X size={20} />
              </button>
            </div>
            <p className="identity-hint">
              По одному студенту на строку: ФИО;КОД. Код — 4–8 латинских букв
              или цифр, уникальный во всей платформе. Полных тёзок различают
              разные коды. Загрузка добавляет записи; повтор существующей строки
              безопасен.
            </p>
            <textarea
              className="roster-input mono"
              aria-label="ФИО и коды"
              rows={5}
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder={
                "Иванов Иван Иванович;AB12CD\nПетрова Анна Сергеевна;XY34EF"
              }
            />
            <input
              className="visually-hidden"
              ref={fileInput}
              type="file"
              accept=".json,application/json"
              aria-label="Загрузить список JSON"
              onChange={async (e) => {
                const file = e.target.files?.[0];
                if (file) {
                  if (file.size > 1048576) {
                    setError("Файл превышает 1 МБ");
                    return;
                  }
                  setText(await file.text());
                }
                e.target.value = "";
              }}
            />
            <div className="roster-actions">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => fileInput.current?.click()}
              >
                <Upload size={14} />
                Открыть JSON
              </Button>
              <Button
                disabled={busy || !text.trim()}
                size="sm"
                onClick={importRoster}
              >
                Загрузить список
              </Button>
              <Button
                variant="ghost"
                size="sm"
                disabled={!invites.data?.length}
                onClick={downloadCodes}
              >
                <Download size={14} />
                Скачать коды
              </Button>
            </div>
            {error && (
              <div className="form-error" role="alert">
                {error}
              </div>
            )}
            {invites.isPending ? (
              <Loading />
            ) : invites.error ? (
              <ErrorState
                error={invites.error}
                retry={() => invites.refetch()}
              />
            ) : (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>ФИО</th>
                      <th>Код</th>
                    </tr>
                  </thead>
                  <tbody>
                    {invites.data?.map((v) => (
                      <tr key={v.id}>
                        <td>{v.full_name}</td>
                        <td className="mono">{v.code}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </div>
      )}
      {clear && (
        <div className="modal-backdrop">
          <div
            className="modal compact-modal"
            role="dialog"
            aria-modal="true"
            aria-label="Завершение сессий"
          >
            <h2>Завершить ученические сессии?</h2>
            <p>
              {clear.group ? `Группа ${clear.group.name}` : "Выбранный студент"}{" "}
              войдёт заново по ФИО и группе, с кодом при необходимости.
              Черновики и история сохранятся.
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
                  setClear(null);
                  setError("");
                }}
              >
                Отмена
              </Button>
              <Button disabled={busy} onClick={clearSessions}>
                Завершить сессии
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
