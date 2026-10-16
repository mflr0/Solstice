#!/usr/bin/env bash
# Imports a Jira database dump into its own Solstice database, for trying
# real data locally.
#
#   scripts/import-dump.sh <dump.sql[.gz]> <name> [time zone]
#
# The dump (MySQL or PostgreSQL, plain SQL, optionally gzipped) is loaded
# into the matching container of compose.yaml as database <name>, then
# imported into the Solstice database solstice_<name>. Serve it with:
#
#   SOLSTICE_DB_URL=mysql://solstice:solstice@localhost:13306/solstice_<name> \
#   SOLSTICE_DATA=data/<name> go run ./cmd/solstice serve        (in backend/)
#
# Dumps stay local: *.sql and *.sql.gz are ignored by git.
set -euo pipefail

dump=${1:?usage: import-dump.sh <dump.sql[.gz]> <name> [time zone]}
name=${2:?usage: import-dump.sh <dump.sql[.gz]> <name> [time zone]}
tz=${3:-Europe/Paris}
root=$(cd "$(dirname "$0")/.." && pwd)
compose=(docker compose -f "$root/compose.yaml")

[[ $name =~ ^[a-z0-9_]+$ ]] || { echo "name: lower-case letters, digits and _ only" >&2; exit 2; }

cat_dump() { if [[ $dump == *.gz ]]; then gzip -dc "$dump"; else cat "$dump"; fi; }

if cat_dump | head -c 4096 | grep -q "PostgreSQL database dump"; then
  echo "› dump PostgreSQL → base $name"
  "${compose[@]}" exec -T jira-pg psql -U jira -d jira -q -c "DROP DATABASE IF EXISTS $name" -c "CREATE DATABASE $name OWNER jira"
  cat_dump | "${compose[@]}" exec -T jira-pg psql -U jira -d "$name" -q -v ON_ERROR_STOP=0 >/dev/null
  source_url="postgres://jira:jira@localhost:15432/$name"
else
  echo "› dump MySQL → base $name"
  "${compose[@]}" exec -T jira-mysql mysql -uroot -proot -e \
    "DROP DATABASE IF EXISTS $name; CREATE DATABASE $name CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
     GRANT ALL ON $name.* TO 'jira'@'%';" 2>/dev/null
  cat_dump | "${compose[@]}" exec -T jira-mysql mysql -uroot -proot --default-character-set=utf8mb4 \
    --init-command="SET SESSION sql_mode=''" "$name" 2>/dev/null
  source_url="mysql://jira:jira@localhost:23306/$name"
fi

echo "› base Solstice solstice_$name"
"${compose[@]}" exec -T mariadb mariadb -uroot -proot -e \
  "CREATE DATABASE IF NOT EXISTS solstice_$name; GRANT ALL ON solstice_$name.* TO 'solstice'@'%';"

echo "› import (fuseau du serveur Jira : $tz)"
cd "$root/backend"
SOLSTICE_DB_URL="mysql://solstice:solstice@localhost:13306/solstice_$name" SOLSTICE_DATA="$root/data/$name" \
  go run ./cmd/solstice import -replace -tz "$tz" -from "$source_url"
