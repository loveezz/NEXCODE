"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  ArrowUpRight,
  BookOpen,
  ChevronDown,
  Code2,
  Command,
  HelpCircle,
  History,
  LayoutDashboard,
  LogOut,
  Moon,
  Search,
  Settings2,
  KeyRound,
  ShieldCheck,
  Sun,
  TrendingUp,
  Users,
} from "lucide-react";
import { api, ApiError, send } from "@/lib/api";
import type { User } from "@/lib/types";
import { useTheme } from "./providers";
import { ErrorState, Loading } from "./common";
import { isPreview } from "@/lib/runtime";
export function useUser() {
  return useQuery({
    queryKey: ["me"],
    queryFn: () => api<User>("/auth/me"),
    retry: false,
  });
}
const nav = [
  { href: "/", label: "Обзор", icon: LayoutDashboard },
  { href: "/labs", label: "Лабораторные", icon: BookOpen },
  { href: "/history", label: "История отправок", icon: History },
  { href: "/progress", label: "Мой прогресс", icon: TrendingUp },
];
export default function Shell({ children }: { children: React.ReactNode }) {
  const { data: user, isPending, error, refetch } = useUser();
  const path = usePathname();
  const router = useRouter();
  const client = useQueryClient();
  const theme = useTheme();
  const [search, setSearch] = useState("");
  useEffect(() => {
    if (error instanceof ApiError && error.status === 401) {
      client.clear();
      router.replace("/login");
    }
  }, [error, router, client]);
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        document
          .querySelector<HTMLInputElement>(".global-search input")
          ?.focus();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);
  const logout = async () => {
    await send("/auth/logout", {});
    client.clear();
    router.push("/login");
  };
  const [logoutError, setLogoutError] = useState("");
  if (isPending) return <Loading text="Подключаем рабочее пространство…" />;
  if (error || !user)
    return (
      <main className="standalone">
        <ErrorState error={error} retry={() => refetch()} />
      </main>
    );
  const section = path.startsWith("/exercise")
    ? "Рабочее пространство"
    : path === "/teacher"
      ? "Студенты"
      : path === "/manage"
        ? "Управление каталогом"
        : path === "/admin"
          ? "Администрирование"
          : path === "/identity"
            ? "Группы и вход"
            : nav.find((n) => n.href === path)?.label || "Обзор";
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Link className="brand" href="/">
          <span className="brand-icon">
            <Code2 size={23} strokeWidth={2.5} />
          </span>
          NEXCODE<span className="brand-dot">®</span>
        </Link>
        <div className="workspace">
          <span className="workspace-icon">N</span>
          <div>
            <strong>Python Workspace</strong>
            <small>Практикум по программированию</small>
          </div>
          <ChevronDown size={14} />
        </div>
        <p className="nav-label">ОБУЧЕНИЕ</p>
        <nav>
          {nav.map((n) => (
            <Link
              href={n.href}
              key={n.href}
              className={`nav-item ${path === n.href ? "active" : ""}`}
            >
              <n.icon size={18} />
              <span>{n.label}</span>
              {path === n.href && <span className="nav-active-dot" />}
            </Link>
          ))}
        </nav>
        {user.role !== "student" && (
          <>
            <p className="nav-label">УПРАВЛЕНИЕ</p>
            <nav>
              <Link
                className={`nav-item ${path === "/teacher" ? "active" : ""}`}
                href="/teacher"
              >
                <Users size={18} />
                Студенты
              </Link>
              <Link
                className={`nav-item ${path === "/manage" ? "active" : ""}`}
                href="/manage"
              >
                <Settings2 size={18} />
                Каталог заданий
              </Link>
              <Link
                className={`nav-item ${path === "/identity" ? "active" : ""}`}
                href="/identity"
              >
                <KeyRound size={18} />
                Группы и вход
              </Link>
              {user.role === "admin" && (
                <Link
                  className={`nav-item ${path === "/admin" ? "active" : ""}`}
                  href="/admin"
                >
                  <ShieldCheck size={18} />
                  Администрирование
                </Link>
              )}
            </nav>
          </>
        )}
        <div className="sidebar-bottom">
          <Link className="help-card" href="/guide">
            <span>
              <HelpCircle size={18} />
              Нужна подсказка?
              <ArrowUpRight size={16} />
            </span>
            <small>Как устроена платформа</small>
          </Link>
          <div className="profile">
            <span className="avatar">
              {user.name.slice(0, 1).toUpperCase()}
            </span>
            <div>
              <strong>{user.name}</strong>
              <small>
                {
                  {
                    student: user.group_name || "Студент",
                    teacher: "Преподаватель",
                    admin: isPreview ? "Предпросмотр" : "Администратор",
                  }[user.role]
                }
              </small>
            </div>
            <button
              className="icon-button"
              aria-label="Выйти"
              title="Выйти"
              onClick={() => logout().catch((e) => setLogoutError(e.message))}
            >
              <LogOut size={16} />
            </button>
          </div>
          {logoutError && <small role="alert">{logoutError}</small>}
        </div>
      </aside>
      <div className="main-area">
        <header className="topbar">
          <div className="breadcrumb">
            Рабочее пространство<span>/</span>
            <strong>{section}</strong>
          </div>
          <div className="top-actions">
            <form
              className="global-search"
              onSubmit={(e) => {
                e.preventDefault();
                router.push(`/labs?q=${encodeURIComponent(search)}`);
              }}
            >
              <Search size={15} />
              <input
                aria-label="Найти упражнение"
                placeholder="Найти упражнение…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
              <kbd>
                <Command size={10} /> K
              </kbd>
            </form>
            <button
              className="icon-button theme-button"
              aria-label="Переключить тему"
              onClick={theme.toggle}
            >
              {theme.dark ? <Sun size={18} /> : <Moon size={18} />}
            </button>
            <span className="top-divider" />
            <span className="environment">
              <span />
              {isPreview ? "Предпросмотр интерфейса" : "Локальное пространство"}
            </span>
          </div>
        </header>
        {isPreview && (
          <div className="preview-banner" role="status">
            <strong>Предварительный просмотр</strong>
            <span>
              Сервер пока не подключён. Запуск кода и сохранение данных
              недоступны.
            </span>
          </div>
        )}
        <main
          className={
            path.startsWith("/exercise")
              ? "page-content editor-content"
              : "page-content"
          }
        >
          {children}
        </main>
        <footer className="footer">
          <span>
            NEXCODE <span>·</span> Практика превращается в навык.
          </span>
          <span>
            Python 3.12 <span className="footer-dot" /> Go Engine
          </span>
        </footer>
      </div>
    </div>
  );
}
