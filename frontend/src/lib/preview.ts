import catalogData from "@/data/preview-catalog.json";
import type { Catalog, User } from "./types";
export const previewMessage =
  "Это предварительный просмотр интерфейса. Подключите сервер для сохранения данных и выполнения Python.";
const catalog: Catalog = catalogData as Catalog;
const guest: User = {
  id: 0,
  login: "preview",
  name: "Гость",
  full_name: "Гость",
  group_name: "",
  role: "admin",
  blocked: false,
  created_at: "",
  invite_code_id: null,
  last_login_at: "",
  last_login_ip: "",
};
export function previewResponse(path: string, method: string): unknown {
  const pathname = path.split("?")[0];
  if (method === "POST" && pathname === "/auth/logout") return { ok: true };
  if (method !== "GET") throw new Error(previewMessage);
  if (pathname === "/auth/me") return guest;
  if (pathname === "/auth/options")
    return { group_mode: "free", groups: [], semester_end: "" };
  if (pathname === "/catalog" || pathname === "/teacher/catalog")
    return catalog;
  if (/^\/exercises\/\d+\/draft$/.test(pathname))
    return { code: "", exists: false, updated_at: "" };
  const match = pathname.match(/^\/exercises\/(\d+)$/);
  if (match) {
    const exercise = catalog.exercises.find((e) => e.id === Number(match[1]));
    if (!exercise) throw new Error("Упражнение не найдено в каталоге практик.");
    return {
      exercise,
      example: exercise.tests?.trim()
        ? JSON.parse(exercise.tests.split("\n")[0])
        : { stdin: "", stdout: "" },
    };
  }
  if (
    pathname === "/submissions" ||
    pathname === "/teacher/students" ||
    pathname === "/admin/users" ||
    /^\/teacher\/groups\/\d+\/invites$/.test(pathname)
  )
    return [];
  if (pathname === "/admin/metrics")
    return {
      queued: 0,
      running: 0,
      active_sessions: 0,
      users: 0,
      submissions: 0,
    };
  if (pathname === "/teacher/identity")
    return { group_mode: "free", groups: [], semester_end: "" };
  if (pathname === "/teacher/login-events")
    return { events: [], next_before: 0 };
  throw new Error(previewMessage);
}
