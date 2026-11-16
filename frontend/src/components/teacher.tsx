"use client";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { ArrowUpRight, Download, Search, Users } from "lucide-react";
import { api } from "@/lib/api";
import type { Student } from "@/lib/types";
import { useUser } from "./shell";
import { Empty, ErrorState, Loading, Progress, Status } from "./common";
import { Button } from "./ui/button";
import { apiUrl, isPreview } from "@/lib/runtime";
export default function TeacherView() {
  const { data: user } = useUser();
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<number | null>(null);
  const allowed = !!user && user.role !== "student";
  const students = useQuery({
    queryKey: ["students"],
    queryFn: () => api<Student[]>("/teacher/students"),
    enabled: allowed,
    refetchInterval: 15000,
  });
  if (user && !allowed)
    return (
      <ErrorState
        error={
          new Error("Этот раздел доступен преподавателям и администраторам.")
        }
      />
    );
  const visible = students.data?.filter((s) =>
    `${s.user.full_name} ${s.user.group_name}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  return (
    <div>
      <div className="page-heading">
        <div>
          <div className="eyebrow">КАБИНЕТ ПРЕПОДАВАТЕЛЯ</div>
          <h1>Прогресс студентов</h1>
          <p>Наблюдайте за практикой и помогайте на следующем шаге.</p>
        </div>
        {isPreview ? (
          <Button variant="secondary" disabled>
            <Download size={16} />
            Экспорт CSV
          </Button>
        ) : (
          <Button asChild variant="secondary">
            <a href={apiUrl("/teacher/export")} download>
              <Download size={16} />
              Экспорт CSV
            </a>
          </Button>
        )}
      </div>
      <div className="catalog-toolbar">
        <div className="search-field">
          <Search size={17} />
          <input
            aria-label="Найти студента"
            placeholder="ФИО или группа студента…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <span>
          <Users size={16} />
          {students.data?.length || 0} студентов
        </span>
      </div>
      {students.isPending ? (
        <Loading />
      ) : students.error ? (
        <ErrorState error={students.error} retry={() => students.refetch()} />
      ) : !visible?.length ? (
        <Empty title="Студентов пока нет">
          После первого входа студенты появятся здесь автоматически.
        </Empty>
      ) : (
        <div className="student-cards">
          {visible.map((s) => (
            <section className="student-card" key={s.user.id}>
              <div className="student-card-header">
                <span className="avatar">{s.user.name[0]}</span>
                <div>
                  <h3>{s.user.name}</h3>
                  <small>{s.user.group_name}</small>
                </div>
                <strong>
                  {s.solved}
                  <span> / {s.total}</span>
                </strong>
              </div>
              <Progress value={s.total ? (s.solved / s.total) * 100 : 0} />
              <div className="student-card-actions">
                <button
                  onClick={() =>
                    setSelected(selected === s.user.id ? null : s.user.id)
                  }
                >
                  {selected === s.user.id
                    ? "Свернуть упражнения"
                    : "Прогресс по упражнениям"}
                </button>
                <Link href={`/history?all=1&student=${s.user.id}`}>
                  Отправки
                  <ArrowUpRight size={14} />
                </Link>
                <Link href={`/identity?student=${s.user.id}`}>
                  Журнал входов
                  <ArrowUpRight size={14} />
                </Link>
              </div>
              {selected === s.user.id && (
                <div className="student-exercises">
                  {s.progress.map((p) => (
                    <div key={p.exercise_id}>
                      <span>{p.title}</span>
                      <Status status={p.status} />
                    </div>
                  ))}
                </div>
              )}
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
