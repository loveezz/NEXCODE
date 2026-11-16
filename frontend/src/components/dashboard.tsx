"use client";
import Link from "next/link";
import { exerciseHref } from "@/lib/runtime";
import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import {
  ArrowRight,
  ArrowUpRight,
  BookOpen,
  Braces,
  CheckCheck,
  ChevronRight,
  Clock3,
  Code2,
  Flag,
  Play,
  Sparkles,
  Terminal,
  Zap,
} from "lucide-react";
import { api, date, solved } from "@/lib/api";
import type { Catalog, Submission } from "@/lib/types";
import { useUser } from "./shell";
import { Empty, ErrorState, Loading, Progress, Status } from "./common";
import { Button } from "./ui/button";
const labIcons = [Terminal, Braces, Code2, Zap];
export function useCatalog(initial?: Catalog) {
  return useQuery({
    queryKey: ["catalog"],
    queryFn: () => api<Catalog>("/catalog"),
    initialData: initial,
  });
}
export default function Dashboard({ initial }: { initial?: Catalog }) {
  const { data, error, isPending, refetch } = useCatalog(initial);
  const { data: user } = useUser();
  const history = useQuery({
    queryKey: ["history"],
    queryFn: () => api<Submission[]>("/submissions"),
  });
  if (isPending) return <Loading />;
  if (error || !data)
    return <ErrorState error={error} retry={() => refetch()} />;
  const done = data.exercises.filter((e) => solved(e.status)).length;
  const started = data.exercises.filter(
    (e) => e.status === "in_progress",
  ).length;
  const next =
    data.exercises.find((e) => e.status === "in_progress") ||
    data.exercises.find((e) => !solved(e.status)) ||
    data.exercises[0];
  const percent = data.exercises.length
    ? Math.round((done / data.exercises.length) * 100)
    : 0;
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
    >
      <div className="page-heading">
        <div>
          <div className="eyebrow">ВАШЕ РАБОЧЕЕ ПРОСТРАНСТВО</div>
          <h1>
            Привет, {user?.name.split(" ")[0] || "студент"}{" "}
            <span className="wave">✳</span>
          </h1>
          <p>Новая строка кода — ещё один шаг вперёд.</p>
        </div>
        <span className="semester">
          <span />
          Практикум Python
        </span>
      </div>
      <section className="hero">
        <div className="hero-copy">
          <span className="hero-label">
            <span />
            LEARN. CODE. REPEAT.
          </span>
          <h2>
            Меньше теории.
            <br />
            Больше <span>вашего кода.</span>
          </h2>
          <p>
            Решайте задачи, экспериментируйте и получайте
            <br className="desktop-br" /> обратную связь. Всё нужное — уже
            здесь.
          </p>
          {next && (
            <Button asChild>
              <Link href={exerciseHref(next.id)}>
                <Play size={15} fill="currentColor" />
                {started ? "Продолжить практику" : "Начать практику"}
                <ArrowUpRight size={17} />
              </Link>
            </Button>
          )}
          <small>В своём темпе. С понятным результатом.</small>
        </div>
        <div className="hero-art" aria-hidden="true">
          <div className="art-orbit orbit-one" />
          <div className="art-orbit orbit-two" />
          <span className="art-star star-one">✳</span>
          <span className="art-star star-two">+</span>
          <div className="hero-code">
            <div className="code-title">
              <span className="window-dots">
                <i />
                <i />
                <i />
              </span>
              <span>your_next_step.py</span>
              <span>↗</span>
            </div>
            <div className="code-lines">
              <p>
                <i>1</i>
                <span className="syntax-purple">def</span>{" "}
                <span className="syntax-yellow">keep_learning</span>():
              </p>
              <p>
                <i>2</i> progress = <span className="syntax-orange">0</span>
              </p>
              <p>
                <i>3</i> <span className="syntax-purple">while</span>{" "}
                <span className="syntax-orange">True</span>:
              </p>
              <p>
                <i>4</i> practice()
              </p>
              <p>
                <i>5</i> progress += <span className="syntax-orange">1</span>
              </p>
              <p>
                <i>6</i>{" "}
                <span className="syntax-muted"># у тебя получится</span>
                <span className="code-cursor" />
              </p>
            </div>
            <div className="code-status">
              <span />
              Python 3.12<span>UTF-8</span>
            </div>
          </div>
          <div className="art-badge">
            <CheckCheck size={15} />
            <span>Каждая попытка — это опыт</span>
          </div>
        </div>
      </section>
      <section className="stats-grid">
        <div className="stat-card">
          <span className="stat-icon orange">
            <BookOpen size={19} />
          </span>
          <div>
            <p>Лабораторных работ</p>
            <strong>
              {data.labs.length}
              <small>в вашем каталоге</small>
            </strong>
          </div>
          <ArrowUpRight size={16} />
        </div>
        <div className="stat-card">
          <span className="stat-icon green">
            <CheckCheck size={19} />
          </span>
          <div>
            <p>Упражнений решено</p>
            <strong>
              {done}
              <small>из {data.exercises.length} доступных</small>
            </strong>
          </div>
          <span className="stat-mini">{percent}%</span>
        </div>
        <div className="stat-card">
          <span className="stat-icon blue">
            <Clock3 size={19} />
          </span>
          <div>
            <p>Сейчас в работе</p>
            <strong>
              {started}
              <small>продолжайте с любого</small>
            </strong>
          </div>
          <span className="tiny-bars">
            <i />
            <i />
            <i />
            <i />
            <i />
          </span>
        </div>
      </section>
      <div className="dashboard-columns">
        <section>
          <div className="section-heading">
            <h2>
              Ваши лабораторные{" "}
              <span>{data.labs.length.toString().padStart(2, "0")}</span>
            </h2>
            <Link href="/labs">
              Все работы
              <ArrowRight size={15} />
            </Link>
          </div>
          <div className="lab-grid">
            {data.labs.slice(0, 4).map((lab, i) => {
              const es = data.exercises.filter((e) => e.lab_id === lab.id);
              const complete = es.filter((e) => solved(e.status)).length;
              const Icon = labIcons[i % 4];
              return (
                <Link
                  href={`/labs?lab=${lab.id}`}
                  key={lab.id}
                  className="lab-card"
                >
                  <div className="lab-card-top">
                    <span className={`lab-symbol lab-symbol-${i % 4}`}>
                      <Icon size={21} />
                    </span>
                    <span className="lab-number">
                      ЛАБОРАТОРНАЯ {String(i + 1).padStart(2, "0")}
                    </span>
                    <ChevronRight size={16} />
                  </div>
                  <h3>{lab.title}</h3>
                  <p>{lab.description}</p>
                  <div className="lab-card-meta">
                    <span>{es.length} упражнения</span>
                    <span>
                      {complete}/{es.length}
                    </span>
                  </div>
                  <Progress
                    value={es.length ? (complete / es.length) * 100 : 0}
                  />
                  <div className="lab-card-bottom">
                    <span>
                      {complete === es.length && es.length
                        ? "Все задания решены"
                        : complete
                          ? "Продолжить обучение"
                          : "Готова к изучению"}
                    </span>
                    {lab.source === "practice" && <small>Практика</small>}
                  </div>
                </Link>
              );
            })}
          </div>
          {data.labs.length === 0 && (
            <Empty title="Каталог пока пуст">
              Преподаватель скоро добавит первые лабораторные работы.
            </Empty>
          )}
        </section>
        <aside className="dashboard-aside">
          <section className="overall-card">
            <div className="section-heading">
              <h2>Общий прогресс</h2>
              <Flag size={16} />
            </div>
            <div
              className="progress-ring"
              style={{
                background: `conic-gradient(var(--accent) ${percent}%, var(--border) 0)`,
              }}
            >
              <div>
                <strong>
                  {percent}
                  <span>%</span>
                </strong>
                <small>практикума пройдено</small>
              </div>
            </div>
            <p>
              {done === 0
                ? "Большой путь начинается\nс первой решённой задачи."
                : `Отличный темп! Решено ${done} из ${data.exercises.length} упражнений.`}
            </p>
            <Link href="/progress">
              Посмотреть прогресс
              <ArrowRight size={14} />
            </Link>
          </section>
          <section className="activity-card">
            <div className="section-heading">
              <h2>Последняя активность</h2>
              <span className="live-dot" />
            </div>
            {history.isError ? (
              <ErrorState
                error={history.error}
                retry={() => history.refetch()}
              />
            ) : !history.data?.length ? (
              <div className="activity-empty">
                <span>
                  <Terminal size={19} />
                </span>
                <strong>Здесь начинается история</strong>
                <p>
                  Первая отправка появится здесь.
                  <br />
                  Попробуйте решить упражнение!
                </p>
              </div>
            ) : (
              <div className="activity-list">
                {history.data.slice(0, 3).map((s) => (
                  <Link href={`/history?id=${s.id}`} key={s.id}>
                    <span
                      className={`activity-dot ${solved(s.status) ? "green-dot" : ""}`}
                    />
                    <div>
                      <strong>{s.title}</strong>
                      <small>{date(s.created_at)}</small>
                      <Status status={s.status} />
                    </div>
                    <ChevronRight size={13} />
                  </Link>
                ))}
              </div>
            )}
          </section>
        </aside>
      </div>
      <div className="tip-strip">
        <span>
          <Sparkles size={17} />
        </span>
        <p>
          <strong>Код не обязан получаться с первого раза.</strong> Проверяйте
          гипотезы, читайте результаты тестов и пробуйте снова.
        </p>
        <Link href="/guide">
          Как это работает
          <ArrowUpRight size={14} />
        </Link>
      </div>
    </motion.div>
  );
}
