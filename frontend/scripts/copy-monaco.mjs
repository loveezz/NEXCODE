import { cpSync, mkdirSync } from "node:fs";
mkdirSync(new URL("../public/monaco", import.meta.url), { recursive: true });
cpSync(
  new URL("../node_modules/monaco-editor/min/vs", import.meta.url),
  new URL("../public/monaco/vs", import.meta.url),
  { recursive: true },
);
