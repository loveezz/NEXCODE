import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import { existsSync, writeFileSync } from "node:fs";
const require = createRequire(import.meta.url);

export function buildStatic({ platform, basePath }) {
  const endpoint = (process.env.NEXT_PUBLIC_API_URL || "").trim();
  if (endpoint && !/^https:\/\//i.test(endpoint))
    throw new Error("Static hosting requires an HTTPS API URL.");
  const environment = {
    ...process.env,
    NEXT_TELEMETRY_DISABLED: "1",
    NEXCODE_STATIC_EXPORT: "true",
    GITHUB_PAGES: platform === "GitHub Pages" ? "true" : "false",
    NEXT_PUBLIC_BASE_PATH: basePath,
    NEXT_PUBLIC_PREVIEW_MODE: endpoint ? "false" : "true",
  };
  const result = spawnSync(
    process.execPath,
    [require.resolve("next/dist/bin/next"), "build"],
    { stdio: "inherit", env: environment },
  );
  if (result.status !== 0) process.exit(result.status || 1);
  if (!existsSync("out/index.html"))
    throw new Error(
      "Static export did not create out/index.html. Do not deploy the repository root.",
    );
  if (platform === "GitHub Pages") writeFileSync("out/.nojekyll", "");
  console.log(
    `${platform} export: ${endpoint ? "external API configured" : "clearly marked, read-only interface preview"}; publish the out directory.`,
  );
}
