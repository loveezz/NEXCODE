export const basePath = (process.env.NEXT_PUBLIC_BASE_PATH || "").replace(
  /\/$/,
  "",
);
export const isPreview = process.env.NEXT_PUBLIC_PREVIEW_MODE === "true";
const apiBase = (process.env.NEXT_PUBLIC_API_URL || `${basePath}/api`).replace(
  /\/$/,
  "",
);
export const apiUrl = (path: string) => `${apiBase}${path}`;
export const exerciseHref = (id: number) => `/exercise?id=${id}`;
