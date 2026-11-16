import { buildStatic } from "./build-static.mjs";
buildStatic({
  platform: "GitHub Pages",
  basePath: process.env.NEXT_PUBLIC_BASE_PATH ?? "/NEXCODE",
});
