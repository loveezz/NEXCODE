import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import { writeFileSync } from "node:fs";
const require = createRequire(import.meta.url);
const endpoint = (process.env.NEXT_PUBLIC_API_URL || "").trim();
if (endpoint && !/^https:\/\//i.test(endpoint))
  throw new Error("GitHub Pages requires an HTTPS API URL.");
const environment = {
  ...process.env,
  NEXT_TELEMETRY_DISABLED: "1",
  GITHUB_PAGES: "true",
  NEXT_PUBLIC_BASE_PATH: process.env.NEXT_PUBLIC_BASE_PATH ?? "/NEXCODE",
  NEXT_PUBLIC_PREVIEW_MODE: endpoint ? "false" : "true",
};
const result = spawnSync(
  process.execPath,
  [require.resolve("next/dist/bin/next"), "build"],
  { stdio: "inherit", env: environment },
);
if (result.status !== 0) process.exit(result.status || 1);
writeFileSync("out/.nojekyll", "");
console.log(
  endpoint
    ? "GitHub Pages export uses the configured external API."
    : "GitHub Pages export is a clearly marked, read-only interface preview.",
);
