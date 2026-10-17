#!/usr/bin/env bash
# Rebuilds backend/tests/testdata/seed.sql, the Solstice database the CI
# integration tests load (.github/workflows/integration.yml). CI has no Jira
# distribution to generate data from, so the dump of a synthetic import is
# committed instead: Solstice's own schema, synthetic data only.
#
# What it does: runs the synthetic import (scripts/e2e-import.sh --import-only),
# then dumps the MySQL-sourced result (solstice_e2e_my) with mariadb-dump, in a
# stable form: no dump date, no comments, rows in primary key order, one row per
# INSERT unless that makes the file larger than 2 MB. It then checks that the
# seed is synthetic.
#
# When to run it: after changing the fixture (backend/internal/jirafixture) or
# the migrations. The seed may lag behind the migrations: CI migrates it after
# loading it. Commit the new file with the change that required it.
#
# What it needs: the containers of compose.yaml (docker compose up -d) and the
# Jira distribution (JIRA_DIST, see e2e-import.sh).
#
#   JIRA_DIST=/path/to/atlassian-jira-software-X-standalone ./scripts/make-test-seed.sh
#   ./scripts/make-test-seed.sh --check    # only check the committed seed (no containers)
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
seed="$root/backend/tests/testdata/seed.sql"
max=$((2 * 1024 * 1024))

# The fixture gives every account an @exemple.fr address; any other e-mail
# domain in the seed means real data slipped in.
check_synthetic() {
  local other
  other=$(grep -oE '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' "$seed" | grep -viE '@exemple\.fr$' | sort -u || true)
  if [ -n "$other" ]; then
    echo "the seed holds e-mail addresses that are not the fixture's (@exemple.fr), it is not synthetic:" >&2
    echo "$other" | sed 's/^[^@]*/…/' >&2
    return 1
  fi
  echo "› the seed only holds @exemple.fr addresses"
}

if [ "${1:-}" = "--check" ]; then check_synthetic; exit; fi

"$root/scripts/e2e-import.sh" --import-only

dump() { # mariadb-dump options...
  docker compose -f "$root/compose.yaml" exec -T mariadb mariadb-dump -uroot -proot \
    --single-transaction --skip-comments --skip-dump-date --skip-add-locks --order-by-primary "$@" solstice_e2e_my
}

echo "› dump of solstice_e2e_my"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
dump --skip-extended-insert > "$tmp"
if [ "$(wc -c < "$tmp")" -gt "$max" ]; then
  echo "  more than 2 MB with one row per INSERT, using extended inserts"
  dump > "$tmp"
fi

mkdir -p "$(dirname "$seed")"
mv "$tmp" "$seed"
echo "  $(wc -c < "$seed" | tr -d ' ') bytes"
check_synthetic
