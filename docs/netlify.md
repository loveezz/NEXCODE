# Netlify

Netlify должен публиковать **готовый статический сайт**, а не корень исходников. Если README.md доступен по URL, а `/` отвечает Page not found, в publish directory попала папка проекта без index.html.

## Автоматически из GitHub

В корне есть [netlify.toml](../netlify.toml), который задаёт:

| Настройка | Значение |
|---|---|
| Base directory | `frontend` |
| Build command | `npm run build:netlify` |
| Publish directory | `out` относительно frontend, то есть `frontend/out` от корня репозитория |
| Node.js | `24` |

Подключите репозиторий, ветку main и запустите новый deploy. Конфигурация из файла имеет приоритет над соответствующими полями Netlify UI. Статический сайт не требует Next.js server adapter; NETLIFY_NEXT_PLUGIN_SKIP установлен в конфигурации.

## Загрузка вручную

Netlify при ручной загрузке папки не выполняет npm build. Сначала соберите проект:

```sh
cd frontend
npm ci
npm run build:netlify
```

Загрузите **папку frontend/out**, в которой index.html находится непосредственно в корне. Не загружайте корень NEXCODE, frontend, .next или сборку build:pages. При использовании ZIP распакуйте его и загрузите папку с index.html в раздел ручного deploy существующего сайта.

Сборка Netlify всегда использует пустой basePath: `/`, `/_next/`, `/monaco/`. Сборка GitHub Pages использует `/NEXCODE/`; эти два результата не взаимозаменяемы. В out также записываются _headers и правило совместимости `/exercise/1` → `/exercise/?id=1`. Страницы `/labs/`, `/login/`, `/exercise/` имеют собственные index.html и работают после перезагрузки; общий SPA rewrite на index.html не нужен.

Пока NEXT_PUBLIC_API_URL не настроен, опубликован явный предварительный просмотр. Реальные сессии, сохранение и Python требуют отдельного API, как описано в [инструкции по статическому размещению](github-pages.md#подключение-api-позже).
