import { writeFileSync } from "node:fs";
import { buildStatic } from "./build-static.mjs";

// Netlify serves this site at the domain root, even if a previous Pages variable remains configured.
buildStatic({ platform: "Netlify", basePath: "" });
writeFileSync("out/_redirects", "/exercise/:id /exercise/?id=:id 301\n");
writeFileSync(
  "out/_headers",
  "/*\n  X-Content-Type-Options: nosniff\n  X-Frame-Options: DENY\n  Referrer-Policy: same-origin\n",
);
