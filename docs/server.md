# Развёртывание на Linux-сервере

Полная версия NEXCODE работает на сервере с Docker Engine и Compose v2.
Ориентир: 2 vCPU, 4 ГБ RAM, свободные TCP-порты 80 и 443.
Используйте отдельную папку, например `/opt/nexcode`; `.env` не публикуется в Git.

1. Скопируйте исходники и выполните `sh scripts/setup.sh`. Сохраните показанный
   пароль администратора. На диске в `.env` остаётся только bcrypt-хэш.
2. В `.env` задайте публичный адрес и HTTPS:

   ```dotenv
   SITE_ADDRESS=YOUR_PUBLIC_IP_OR_DOMAIN
   APP_URL=https://YOUR_PUBLIC_IP_OR_DOMAIN
   COOKIE_SECURE=true
   BIND_ADDRESS=127.0.0.1
   PORT=3000
   ```

   Домен должен указывать на сервер. Можно использовать непосредственно публичный
   IP: Caddy запрашивает доверенный сертификат Let's Encrypt с профилем
   `shortlived`. Сертификаты автоматически продлеваются и хранятся в отдельном
   Docker-томе. Для проверки адреса сервер должен быть доступен извне на 80/443.

3. Запустите:

   ```sh
   docker compose -f docker-compose.yml -f deploy/compose.production.yml up --build -d
   docker compose -f docker-compose.yml -f deploy/compose.production.yml ps
   ```

Откройте `APP_URL`. Студенческий вход находится на `/login`, служебный —
на `/staff/login`, логин администратора по умолчанию `admin`.
API и Python-исполнитель работают на этом же сервере, режим предварительного
просмотра в обычной Docker-сборке отключён.

HTTPS-прокси направляет API непосредственно в Go и заменяет входящие IP-заголовки
адресом соединения. Backend доверяет заголовкам только внутри настроенной
Docker-подсети `WEB_SUBNET`. Runner и Docker-сокет не публикуются наружу.
Внутренний HTTP gateway остаётся доступен только через localhost:3000.

Для небольшого тестового сервера (1 vCPU / 2 ГБ RAM) добавьте последним файлом
`-f deploy/compose.small.yml`. Он ограничивает память сервисов и оставляет один
воркер исполнения Python. Для сборки на таком сервере полезен swap. Требование
30 одновременных пользователей на этой конфигурации не подтверждено.

Если на сервере уже занят localhost:3000, задайте свободный `PORT` в `.env`.
Если порты 80/443 заняты другим HTTPS-прокси, подключите его к локальному gateway
и запускайте основной compose без `compose.production.yml`.
Настройте доверенный IP на gateway перед проксированием реальных адресов клиентов.

## Проверка

```sh
curl --fail https://YOUR_PUBLIC_IP_OR_DOMAIN/api/health
python3 -m unittest discover -s sandbox -v
docker compose -f docker-compose.yml -f deploy/compose.production.yml logs --tail=60 backend runner https
```

Дополнительно войдите как студент, запустите `print("NEXCODE")`, проверьте
упражнение с автотестами и повторный вход с восстановлением черновика.
HTTPS-порты и `/api/health` должны быть доступны с другого компьютера.

Для обновления повторите команду `up --build -d` из той же папки. Настройка
`restart: unless-stopped` восстанавливает контейнеры после перезапуска Docker.
Включите автозапуск самого Docker средствами ОС.

Перед переносом или обновлением делайте резервную копию SQLite через Online
Backup API либо при остановленном backend. Команда `docker compose down`
сохраняет тома; `down -v` удаляет базу, журналы и сертификаты.

IP-сертификаты: [Let's Encrypt](https://letsencrypt.org/2026/01/15/6day-and-ip-general-availability).
Профиль ACME: [документация Caddy](https://caddyserver.com/docs/caddyfile/directives/tls#issuers).
