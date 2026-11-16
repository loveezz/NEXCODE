"use client";
import { useEffect, useId, useState } from "react";
import { useTheme } from "./providers";

export default function DiagramPreview({ text }: { text: string }) {
  const { dark } = useTheme();
  const id = useId().replace(/[^a-zA-Z0-9]/g, "");
  const [svg, setSvg] = useState("");
  const [error, setError] = useState("");
  const fence = String.fromCharCode(96).repeat(3);
  const fenced = text.match(
    new RegExp(fence + "mermaid\\s*\\n([\\s\\S]*?)" + fence, "i"),
  );
  const source =
    fenced?.[1] ||
    (/^\s*(flowchart|graph)\s+(TD|TB|LR|RL|BT)\b/.test(text) ? text : "");
  useEffect(() => {
    let active = true;
    setSvg("");
    setError("");
    if (!source) return;
    const timer = setTimeout(async () => {
      try {
        if (source.length > 16000) throw new Error("Схема слишком большая");
        const mermaid = (await import("mermaid")).default;
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          theme: dark ? "dark" : "default",
          maxTextSize: 16000,
          maxEdges: 250,
          suppressErrorRendering: true,
          flowchart: { htmlLabels: false },
        });
        const result = await mermaid.render("nexcode-diagram-" + id, source);
        if (active) setSvg(result.svg);
      } catch {
        if (active)
          setError(
            "Проверьте синтаксис Mermaid. Ответ можно сохранить и без предпросмотра.",
          );
      }
    }, 700);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [source, dark, id]);
  if (!source) return null;
  return (
    <section className="diagram-preview" aria-label="Предпросмотр блок-схемы">
      <h4>Блок-схема</h4>
      {error ? (
        <p role="status">{error}</p>
      ) : svg ? (
        <div dangerouslySetInnerHTML={{ __html: svg }} />
      ) : (
        <p>Строим схему…</p>
      )}
    </section>
  );
}
