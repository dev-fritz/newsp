#!/bin/sh
# Creates .env from .env.example with freshly generated secrets.
set -eu

cd "$(dirname "$0")/.."

if [ -e .env ]; then
  echo "init-env: .env already exists, refusing to overwrite it" >&2
  exit 1
fi
command -v openssl >/dev/null 2>&1 || { echo "init-env: openssl is required" >&2; exit 1; }

jwt=$(openssl rand -base64 48 | tr -d '\n')
enc=$(openssl rand -base64 32 | tr -d '\n')
session=$(openssl rand -base64 48 | tr -d '\n')
pg=$(openssl rand -hex 24)

# base64 may contain '/', '+' and '=': use '|' as the sed delimiter and escape '&'.
esc() { printf '%s' "$1" | sed 's/[&|\\]/\\&/g'; }

umask 077
sed \
  -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$(esc "$pg")|" \
  -e "s|^JWT_SECRET=.*|JWT_SECRET=$(esc "$jwt")|" \
  -e "s|^ENCRYPTION_KEY=.*|ENCRYPTION_KEY=$(esc "$enc")|" \
  -e "s|^SESSION_SECRET=.*|SESSION_SECRET=$(esc "$session")|" \
  .env.example > .env

echo "init-env: wrote .env"
