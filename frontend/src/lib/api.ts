import { apiUrl, isPreview } from "./runtime";
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}
export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  if (isPreview) {
    const { previewResponse } = await import("./preview");
    try {
      return previewResponse(path, options?.method || "GET") as T;
    } catch (error) {
      throw new ApiError(503, (error as Error).message);
    }
  }
  let response: Response;
  try {
    response = await fetch(apiUrl(path), {
      ...options,
      credentials: process.env.NEXT_PUBLIC_API_URL ? "include" : "same-origin",
      headers: { "Content-Type": "application/json", ...options?.headers },
    });
  } catch {
    throw new ApiError(
      0,
      "Нет связи с сервером. Проверьте подключение и повторите попытку.",
    );
  }
  const body = await response.json().catch(() => ({}));
  if (!response.ok)
    throw new ApiError(
      response.status,
      body.error || `Ошибка сервера (${response.status})`,
    );
  return body as T;
}
export function send<T>(path: string, body: unknown, method = "POST") {
  return api<T>(path, { method, body: JSON.stringify(body) });
}
export const solved = (status: string) =>
  status === "passed" || status === "warnings";
export const statusLabels: Record<string, string> = {
  not_started: "Не начато",
  in_progress: "В работе",
  queued: "В очереди",
  running: "Проверяется",
  passed: "Решено",
  warnings: "Решено с замечаниями",
  failed: "Есть ошибки",
  error: "Ошибка исполнения",
  executed: "Выполнено",
  review: "На проверке преподавателя",
  revision: "Нужна доработка",
};
export const date = (v: string) =>
  new Date(v).toLocaleString("ru-RU", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
