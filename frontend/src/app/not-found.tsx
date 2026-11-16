import Link from "next/link";
export default function NotFound() {
  return (
    <div className="standalone empty">
      <div className="empty-mark">404</div>
      <h1>Страница не найдена</h1>
      <Link className="text-link" href="/">
        Вернуться в пространство →
      </Link>
    </div>
  );
}
