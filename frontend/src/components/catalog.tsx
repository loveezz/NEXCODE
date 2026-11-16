"use client";
import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { exerciseHref } from "@/lib/runtime";
import { ArrowRight, BookOpen, Search, Terminal } from "lucide-react";
import type { Catalog } from "@/lib/types";
import { solved } from "@/lib/api";
import { useCatalog } from "./dashboard";
import { Empty, ErrorState, Loading, Progress, Status } from "./common";
export default function CatalogView({
  initial,
  progress = false,
}: {
  initial?: Catalog;
  progress?: boolean;
}) {
  const query = useSearchParams();
  const [search, setSearch] = useState(query.get("q") || "");
  const [lab, setLab] = useState(query.get("lab") || "all");
  const [filter, setFilter] = useState("all");
  const { data, isPending, error, refetch } = useCatalog(initial);
  useEffect(() => {
    setSearch(query.get("q") || "");
    setLab(query.get("lab") || "all");
  }, [query]);
  if (isPending) return <Loading />;
  if (!data || error)
    return <ErrorState error={error} retry={() => refetch()} />;
  const visible = data.exercises.filter(
    (e) =>
      (lab === "all" || e.lab_id === Number(lab)) &&
      (filter === "all" ||
        (filter === "solved" ? solved(e.status) : e.status === filter)) &&
      `${e.title} ${e.statement}`.toLowerCase().includes(search.toLowerCase()),
  );
  const complete = data.exercises.filter((e) => solved(e.status)).length;
  return (
    <div>
      <div className="page-heading">
        <div>
          <div className="eyebrow">
            {progress ? "ШАГ ЗА ШАГОМ" : "ОТ ЗНАНИЙ К ПРАКТИКЕ"}
          </div>
          <h1>
            {progress ? "Мой прогресс" : "Лабораторные работы"}
            <span className="heading-count">{data.labs.length}</span>
          </h1>
          <p>
            {progress
              ? `Вы решили ${complete} из ${data.exercises.length} упражнений. Каждое решение имеет значение.`
              : "Выберите тему и оттачивайте навыки на реальных задачах."}
          </p>
        </div>
        <span className="semester">
          <BookOpen size={15} />
          Python 3.12
        </span>
      </div>
      <div className="catalog-toolbar">
        <div className="search-field">
          <Search size={17} />
          <input
            aria-label="Поиск в каталоге"
            placeholder="Название или ключевое слово…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <select
          aria-label="Лабораторная работа"
          value={lab}
          onChange={(e) => setLab(e.target.value)}
        >
          <option value="all">Все лабораторные</option>
          {data.labs.map((l) => (
            <option value={l.id} key={l.id}>
              {l.title}
            </option>
          ))}
        </select>
        <select
          aria-label="Статус упражнения"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        >
          <option value="all">Все статусы</option>
          <option value="not_started">Не начато</option>
          <option value="in_progress">В работе</option>
          <option value="solved">Решено</option>
        </select>
        <span>{visible.length} упражнений</span>
      </div>
      <div className="catalog-groups">
        {data.labs
          .filter((l) => visible.some((e) => e.lab_id === l.id))
          .map((l) => {
            const all = data.exercises.filter((e) => e.lab_id === l.id);
            const done = all.filter((e) => solved(e.status)).length;
            return (
              <section className="catalog-group" key={l.id}>
                <div className="catalog-lab-header">
                  <span className="lab-symbol">
                    <Terminal size={23} />
                  </span>
                  <div>
                    <small>
                      ЛАБОРАТОРНАЯ {String(l.position).padStart(2, "0")}
                      {l.source === "demo" ? " · ДЕМОНСТРАЦИОННАЯ" : ""}
                    </small>
                    <h2>{l.title}</h2>
                    <p>{l.description}</p>
                  </div>
                  <div className="lab-progress">
                    <strong>
                      {done}
                      <span> / {all.length}</span>
                    </strong>
                    <Progress
                      value={all.length ? (done / all.length) * 100 : 0}
                    />
                  </div>
                </div>
                <div className="exercise-table">
                  {visible
                    .filter((e) => e.lab_id === l.id)
                    .map((e, i) => (
                      <Link
                        href={exerciseHref(e.id)}
                        className="exercise-row"
                        key={e.id}
                      >
                        <span className="exercise-number">
                          {String(i + 1).padStart(2, "0")}
                        </span>
                        <div>
                          <strong>{e.title}</strong>
                          <small>
                            {e.difficulty} <span>·</span> {e.test_count} теста{" "}
                            <span>·</span> {e.timeout_ms / 1000} с
                          </small>
                        </div>
                        <Status status={e.status} />
                        <span className="exercise-open">
                          Открыть
                          <ArrowRight size={15} />
                        </span>
                      </Link>
                    ))}
                </div>
              </section>
            );
          })}
      </div>
      {!visible.length && (
        <Empty title="Упражнений не найдено">
          Измените запрос или выберите другой фильтр.
        </Empty>
      )}
    </div>
  );
}
