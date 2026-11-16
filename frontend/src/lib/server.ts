import { cookies } from "next/headers";
import type { Catalog } from "./types";
export async function catalogOnServer(): Promise<Catalog | undefined> {
  if (process.env.GITHUB_PAGES === "true") return undefined;
  const jar = await cookies();
  try {
    const response = await fetch(
      `${process.env.BACKEND_URL || "http://127.0.0.1:8080"}/api/catalog`,
      {
        headers: { Cookie: jar.toString() },
        cache: "no-store",
        signal: AbortSignal.timeout(5000),
      },
    );
    return response.ok ? await response.json() : undefined;
  } catch {
    return undefined;
  }
}
