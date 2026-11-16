"use client";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import {
  Activity,
  Ban,
  Check,
  Loader2,
  Search,
  ShieldCheck,
  Users,
} from "lucide-react";
import { api, send } from "@/lib/api";
import type { User } from "@/lib/types";
import { useUser } from "./shell";
import { ErrorState, Loading } from "./common";
import { Button } from "./ui/button";
export default function AdminView() {
  const { data: me } = useUser();
  const allowed = me?.role === "admin";
  const client = useQueryClient();
  const [search, setSearch] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<number | null>(null);
  const users = useQuery({
    queryKey: ["users"],
    queryFn: () => api<User[]>("/admin/users"),
    enabled: allowed,
  });
  const metrics = useQuery({
    queryKey: ["metrics"],
    queryFn: () =>
      api<{
        queued: number;
        running: number;
        active_sessions: number;
        users: number;
        submissions: number;
      }>("/admin/metrics"),
    enabled: allowed,
    refetchInterval: 5000,
  });
  async function update(u: User, values: Partial<User>) {
    setBusy(u.id);
    setError("");
    try {
      await send(
        `/admin/users/${u.id}`,
        { role: u.role, blocked: u.blocked, ...values },
        "PATCH",
      );
      client.invalidateQueries({ queryKey: ["users"] });
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  }
  if (me && !allowed)
    return (
      <ErrorState
        error={new Error("Этот раздел доступен только администратору.")}
      />
    );
  return (
    <div>
      <div className="page-heading">
        <div>
          <div className="eyebrow">УПРАВЛЕНИЕ ПЛАТФОРМОЙ</div>
          <h1>Администрирование</h1>
          <p>Пользователи, доступ и состояние исполнителей.</p>
        </div>
        <span className="semester">
          <ShieldCheck size={16} />
          Панель администратора
        </span>
      </div>
      <div className="metrics-grid">
        {[
          { label: "В очереди", value: metrics.data?.queued, icon: Activity },
          { label: "Исполняется", value: metrics.data?.running, icon: Loader2 },
          {
            label: "Активных сессий",
            value: metrics.data?.active_sessions,
            icon: ShieldCheck,
          },
          { label: "Пользователей", value: metrics.data?.users, icon: Users },
        ].map((v) => (
          <div className="metric" key={v.label}>
            <v.icon size={20} />
            <strong>{v.value ?? "—"}</strong>
            <span>{v.label}</span>
          </div>
        ))}
      </div>
      {metrics.error && (
        <ErrorState error={metrics.error} retry={() => metrics.refetch()} />
      )}
      <div className="import-hint">
        <ShieldCheck size={22} />
        <div>
          <strong>Служебные аккаунты настраиваются отдельно</strong>
          <p>
            Преподавателей и администраторов добавляют через конфигурацию с
            bcrypt-хэшами. Ученические записи остаются без паролей. Группы,
            сроки и журнал доступны в разделе «Группы и вход».
          </p>
        </div>
        <Button asChild variant="secondary" size="sm">
          <Link href="/identity">Настройки входа</Link>
        </Button>
      </div>
      <div className="catalog-toolbar">
        <div className="search-field">
          <Search size={17} />
          <input
            aria-label="Найти пользователя"
            placeholder="ФИО, группа или служебный логин…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      </div>
      {error && (
        <div role="alert" className="form-error">
          {error}
        </div>
      )}
      {users.isPending ? (
        <Loading />
      ) : users.error ? (
        <ErrorState error={users.error} retry={() => users.refetch()} />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Пользователь</th>
                <th>Группа / логин</th>
                <th>Роль</th>
                <th>Статус</th>
                <th>Действия</th>
              </tr>
            </thead>
            <tbody>
              {users.data
                ?.filter((u) =>
                  `${u.full_name} ${u.group_name} ${u.login}`
                    .toLowerCase()
                    .includes(search.toLowerCase()),
                )
                .map((u) => (
                  <tr key={u.id}>
                    <td>
                      <div className="user-cell">
                        <span className="avatar">{u.full_name[0]}</span>
                        <strong>
                          {u.full_name}
                          {u.id === me?.id ? " (вы)" : ""}
                        </strong>
                      </div>
                    </td>
                    <td>{u.role === "student" ? u.group_name : u.login}</td>
                    <td>
                      {u.role === "student" ? (
                        <span className="mode-tag">Студент</span>
                      ) : (
                        <select
                          aria-label={`Роль ${u.login}`}
                          value={u.role}
                          disabled={busy === u.id || u.id === me?.id}
                          onChange={(e) =>
                            update(u, { role: e.target.value as User["role"] })
                          }
                        >
                          <option value="teacher">Преподаватель</option>
                          <option value="admin">Администратор</option>
                        </select>
                      )}
                    </td>
                    <td>
                      <span
                        className={`status ${u.blocked ? "status-failed" : "status-passed"}`}
                      >
                        {u.blocked ? "Заблокирован" : "Активен"}
                      </span>
                    </td>
                    <td>
                      <div className="table-actions">
                        {u.role === "student" && (
                          <Button asChild variant="secondary" size="sm">
                            <Link href={`/identity?student=${u.id}`}>
                              Журнал и сессии
                            </Link>
                          </Button>
                        )}
                        <button
                          className="icon-button"
                          aria-label={
                            u.blocked
                              ? `Разблокировать ${u.full_name}`
                              : `Заблокировать ${u.full_name}`
                          }
                          disabled={busy === u.id || u.id === me?.id}
                          onClick={() => update(u, { blocked: !u.blocked })}
                        >
                          {u.blocked ? <Check size={17} /> : <Ban size={17} />}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
