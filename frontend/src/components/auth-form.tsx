"use client";
import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  Code2,
  Eye,
  EyeOff,
  Loader2,
  Terminal,
} from "lucide-react";
import { api, send } from "@/lib/api";
import type { LoginOptions } from "@/lib/types";
import { ErrorState, Loading } from "./common";
import { Button } from "./ui/button";
import { isPreview } from "@/lib/runtime";
const normalize = (v: string) =>
  v.normalize("NFC").trim().replace(/\s+/gu, " ").toUpperCase();
export default function AuthForm({ staff = false }: { staff?: boolean }) {
  const router = useRouter();
  const client = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [show, setShow] = useState(false);
  const [group, setGroup] = useState("");
  const options = useQuery({
    queryKey: ["login-options"],
    queryFn: () => api<LoginOptions>("/auth/options"),
    enabled: !staff,
    staleTime: 0,
  });
  const chosen = options.data?.groups.find(
    (g) => normalize(g.name) === normalize(group),
  );
  const requiresCode = !!chosen?.invite_required;
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    let body: unknown;
    if (staff) {
      body = {
        login: String(form.get("login") || "").trim(),
        password: String(form.get("password") || ""),
      };
    } else {
      const fullName = normalize(String(form.get("full_name") || ""));
      const groupName = normalize(group);
      const code = String(form.get("code") || "")
        .trim()
        .toUpperCase();
      if (
        !fullName ||
        Array.from(fullName).length > 150 ||
        !groupName ||
        Array.from(groupName).length > 20
      ) {
        setError("Укажите ФИО (до 150 символов) и группу (до 20 символов).");
        return;
      }
      if (requiresCode && !/^[A-Z0-9]{4,8}$/.test(code)) {
        setError(
          "Введите код из 4–8 латинских букв или цифр, выданный преподавателем.",
        );
        return;
      }
      body = {
        full_name: fullName,
        group_name: groupName,
        ...(requiresCode ? { code } : {}),
      };
    }
    setBusy(true);
    try {
      await send(staff ? "/auth/staff/login" : "/auth/login", body);
      client.clear();
      router.replace("/");
      router.refresh();
    } catch (e) {
      setError((e as Error).message);
      if (!staff) options.refetch();
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-page">
      <aside className="auth-story">
        <Link href="/login" className="brand">
          <span className="brand-icon">
            <Code2 size={24} />
          </span>
          NEXCODE<span className="brand-dot">®</span>
        </Link>
        <div>
          <span className="hero-label">
            <span />
            ВАШ СЛЕДУЮЩИЙ ШАГ В PYTHON
          </span>
          <h1>
            Навык приходит
            <br />с <em>практикой.</em>
          </h1>
          <p>
            Ваш код. Ваш прогресс.
            <br />
            Одна платформа для новых открытий.
          </p>
          <div className="auth-code">
            <Terminal size={18} />
            <code>&gt;&gt;&gt; print(&quot;Hello, future!&quot;)</code>
            <span>Hello, future!</span>
          </div>
        </div>
        <small>Продолжайте обучение с любого компьютера в аудитории.</small>
      </aside>
      <main className="auth-main">
        <span className="auth-top-note">
          PYTHON WORKSPACE<span>01 / ∞</span>
        </span>
        <div className="auth-form-wrap">
          <span className="auth-kicker">
            {staff ? "СЛУЖЕБНЫЙ ДОСТУП" : "ДАВАЙТЕ ПИСАТЬ КОД"}
          </span>
          <h2>{staff ? "Вход для сотрудников" : "Вход для студентов"}</h2>
          <p>
            {staff
              ? "Вход для преподавателя и администратора. Учётную запись настраивает администратор."
              : "Укажите ФИО и группу. Ваши решения и прогресс будут доступны с любого компьютера."}
          </p>
          {!staff && options.isPending ? (
            <Loading text="Загружаем группы…" />
          ) : !staff && options.error ? (
            <ErrorState error={options.error} retry={() => options.refetch()} />
          ) : (
            <form onSubmit={submit}>
              {staff ? (
                <>
                  <label>
                    Логин
                    <input
                      name="login"
                      autoComplete="username"
                      required
                      maxLength={100}
                      placeholder="Логин сотрудника"
                    />
                  </label>
                  <label>
                    Пароль
                    <div className="password-field">
                      <input
                        name="password"
                        aria-label="Пароль"
                        type={show ? "text" : "password"}
                        autoComplete="current-password"
                        required
                        maxLength={72}
                        placeholder="Введите пароль"
                      />
                      <button
                        type="button"
                        aria-label={show ? "Скрыть пароль" : "Показать пароль"}
                        onClick={() => setShow(!show)}
                      >
                        {show ? <EyeOff size={17} /> : <Eye size={17} />}
                      </button>
                    </div>
                  </label>
                </>
              ) : (
                <>
                  <label>
                    ФИО
                    <input
                      name="full_name"
                      autoComplete="name"
                      required
                      maxLength={150}
                      placeholder="Иванов Иван Иванович"
                    />
                  </label>
                  <label>
                    Группа
                    {options.data?.group_mode === "list" ? (
                      <select
                        name="group_name"
                        required
                        value={group}
                        onChange={(e) => setGroup(e.target.value)}
                      >
                        <option value="" disabled>
                          Выберите группу
                        </option>
                        {options.data.groups.map((g) => (
                          <option key={g.id} value={g.name}>
                            {g.name}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input
                        name="group_name"
                        autoComplete="organization"
                        list="nexcode-groups"
                        required
                        maxLength={20}
                        value={group}
                        onChange={(e) => setGroup(e.target.value)}
                        placeholder="Например, ПИ-21"
                      />
                    )}
                  </label>
                  <datalist id="nexcode-groups">
                    {options.data?.groups.map((g) => (
                      <option key={g.id} value={g.name} />
                    ))}
                  </datalist>
                  {requiresCode && (
                    <label>
                      Код приглашения
                      <input
                        name="code"
                        aria-label="Код приглашения"
                        aria-describedby="invite-hint"
                        autoComplete="off"
                        required
                        minLength={4}
                        maxLength={8}
                        pattern="[A-Za-z0-9]{4,8}"
                        placeholder="Код от преподавателя"
                      />
                      <small id="invite-hint">
                        Для этой группы включён вход по индивидуальным кодам.
                      </small>
                    </label>
                  )}
                  {options.data?.group_mode === "list" &&
                    options.data.groups.length === 0 && (
                      <p className="form-error">
                        Преподаватель ещё не добавил группы. Обратитесь к нему
                        перед входом.
                      </p>
                    )}
                </>
              )}
              {error && (
                <div className="form-error" role="alert">
                  {error}
                </div>
              )}
              <Button
                type="submit"
                disabled={
                  isPreview ||
                  busy ||
                  (!staff &&
                    options.data?.group_mode === "list" &&
                    options.data.groups.length === 0)
                }
              >
                {busy && <Loader2 size={17} className="spin" />}Войти
                <ArrowRight size={17} />
              </Button>
            </form>
          )}
          {isPreview && (
            <div className="preview-login">
              <p>
                Опубликован предварительный просмотр интерфейса. Вход и
                сохранение данных появятся после подключения сервера.
              </p>
              <Button asChild variant="secondary">
                <Link href="/">Посмотреть интерфейс</Link>
              </Button>
            </div>
          )}
          <Link className="back-login" href={staff ? "/login" : "/staff/login"}>
            {staff
              ? "Вход для студентов"
              : "Вход для преподавателя и администратора"}
          </Link>
          <div className="auth-privacy">
            <span />
            <p>
              {staff
                ? "Служебные аккаунты защищены паролем."
                : "Сессия сохраняется до окончания семестра."}
              <br />
              {staff
                ? "Если доступ потерян, обратитесь к администратору."
                : "На общем компьютере нажимайте «Выйти» после занятия."}
            </p>
          </div>
        </div>
        <footer>
          NEXCODE<span>Практика. Обратная связь. Результат.</span>
        </footer>
      </main>
    </div>
  );
}
