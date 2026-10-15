#!/bin/sh
# Installs Solstice with Docker: random database passwords, both containers
# started, then the address of the setup wizard and its one-time code.
#
#   ./install.sh            (from this directory, or any other)
#
# Running it again keeps the existing .env and data: it only rebuilds and
# restarts (that is also how to upgrade after updating the sources).
set -eu

cd "$(dirname "$0")"

say() { printf '%s\n' "$*"; }
fail() { printf 'Erreur / Error: %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || fail "Docker est requis / Docker is required (https://docs.docker.com/get-docker/)."
docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 est requis / Docker Compose v2 is required."

if [ ! -f .env ]; then
  # Hex only: the password ends up in a database URL.
  secret() { od -An -N24 -tx1 /dev/urandom | tr -d ' \n'; }
  umask 077
  {
    printf 'DB_PASSWORD=%s\n' "$(secret)"
    printf 'DB_ROOT_PASSWORD=%s\n' "$(secret)"
    printf 'SOLSTICE_PORT=%s\n' "${SOLSTICE_PORT:-8080}"
  } > .env
  say "Mots de passe de la base créés dans $(pwd)/.env (gardez ce fichier)."
  say "Database passwords written to $(pwd)/.env (keep this file)."
fi

say "Construction et démarrage… / Building and starting…"
docker compose up -d --build

port=$(sed -n 's/^SOLSTICE_PORT=//p' .env | tail -n 1)
port=${port:-8080}
host=$(hostname 2>/dev/null || echo localhost)

# The setup code is printed when Solstice starts without any account.
code=""
i=0
while [ $i -lt 120 ]; do
  logs=$(docker compose logs --no-color solstice 2>/dev/null || true)
  code=$(printf '%s\n' "$logs" | grep -Eo '[A-Z2-9]{4}-[A-Z2-9]{4}' | tail -n 1 || true)
  [ -n "$code" ] && break
  if docker compose exec -T solstice /usr/local/bin/solstice health >/dev/null 2>&1; then
    # Up: the code, if any, is in the log by now.
    code=$(docker compose logs --no-color solstice 2>/dev/null | grep -Eo '[A-Z2-9]{4}-[A-Z2-9]{4}' | tail -n 1 || true)
    break
  fi
  i=$((i + 1))
  sleep 1
done

say ""
if [ -n "$code" ]; then
  say "Solstice est prêt à être installé / Solstice is ready to be set up:"
  say "  http://$host:$port/setup"
  say "  Code d'installation / Setup code: $code"
else
  say "Solstice tourne / Solstice is running: http://$host:$port/"
  say "(Pas de code : l'instance est déjà installée. / No code: already set up.)"
fi
say ""
say "Journal / Logs:   docker compose -f $(pwd)/compose.yaml logs -f solstice"
say "Arrêt / Stop:     docker compose -f $(pwd)/compose.yaml down"
