#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
umask 077
hash_password() {
  if command -v go >/dev/null 2>&1; then
    (cd backend && go run ./cmd/hash-password)
  else
    docker build --target api -t nexcode-api:local backend >&2
    docker run --rm -i --entrypoint hash-password nexcode-api:local
  fi
}
if [ -f .env ]; then
  if grep -q '^ADMIN_PASSWORD=' .env; then
    if grep -q '^ADMIN_PASSWORD_HASH=.*\$2' .env; then
      sed '/^ADMIN_PASSWORD=/d' .env > .env.tmp
    else
      old_password=$(sed -n 's/^ADMIN_PASSWORD=//p' .env | head -n 1 | tr -d '\r')
      old_password=${old_password#\"}; old_password=${old_password%\"}
      old_password=${old_password#\'}; old_password=${old_password%\'}
      password_hash=$(printf '%s\n' "$old_password" | hash_password)
      sed -e '/^ADMIN_PASSWORD_HASH=/d' -e "s|^ADMIN_PASSWORD=.*|ADMIN_PASSWORD_HASH='$password_hash'|" .env > .env.tmp
      unset old_password
    fi
    mv .env.tmp .env
    echo 'Migrated to bcrypt; existing administrator password still works. Plaintext was removed.'
  else echo '.env already exists; preserved.'; fi
  exit 0
fi
runner_secret=$(openssl rand -hex 32)
initial_password=$(openssl rand -hex 32)
password_hash=$(printf '%s\n' "$initial_password" | hash_password)
sed -e "s|replace-with-another-random-64-hex-secret|$runner_secret|" -e "s|replace-with-bcrypt-hash|$password_hash|" .env.example > .env
echo 'Created .env. Initial staff login: admin.'
printf 'Initial password (shown once; store privately): %s\n' "$initial_password"
echo 'Only the bcrypt hash is stored. Start: docker compose up --build -d'
