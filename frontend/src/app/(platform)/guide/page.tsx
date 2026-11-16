import Link from "next/link";
import { ArrowUpRight, CheckCheck, Cloud, Code2, Terminal } from "lucide-react";
export const metadata = { title: "Как пользоваться" };
export default function Page() {
  return (
    <article className="guide">
      <div className="eyebrow">КОРОТКИЙ МАРШРУТ</div>
      <h1>Ваше пространство для практики</h1>
      <p className="guide-lead">
        Откройте упражнение, напишите решение и узнайте, как оно работает.
      </p>
      <div className="guide-steps">
        {[
          {
            icon: Code2,
            title: "01. Выберите задачу",
            text: "Каталог разделён на лабораторные. В каждой работе видны условия, количество тестов и ограничения.",
          },
          {
            icon: Terminal,
            title: "02. Проверьте идею",
            text: "Пишите Python в редакторе. Кнопка «Запустить» исполняет код с вашими входными данными. Ctrl+Enter тоже запускает программу.",
          },
          {
            icon: CheckCheck,
            title: "03. Отправьте решение",
            text: "Кнопка «Проверить» запускает все тесты. Посмотрите ожидаемый и фактический вывод. Замечания Ruff помогают сделать код аккуратнее.",
          },
          {
            icon: Cloud,
            title: "04. Продолжите позже",
            text: "Черновик сохраняется после двух секунд без изменений. Дождитесь отметки «Черновик сохранён». Все отправки остаются в истории.",
          },
        ].map((s) => (
          <section key={s.title}>
            <s.icon size={25} />
            <div>
              <h2>{s.title}</h2>
              <p>{s.text}</p>
            </div>
          </section>
        ))}
      </div>
      <div className="guide-note">
        <strong>Об учебном каталоге</strong>
        <p>
          Демонстрационные упражнения помечены «Демо». Преподаватель может
          заменить их своими лабораторными через кабинет управления. Автотесты
          NEXCODE — внутренний механизм платформы, а не официальные критерии
          оценивания кафедры.
        </p>
      </div>
      <Link className="text-link" href="/labs">
        Открыть лабораторные
        <ArrowUpRight size={16} />
      </Link>
    </article>
  );
}
