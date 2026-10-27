#!/usr/bin/env bash
# Rebuilds backend/tests/testdata/seed.sql, the Solstice database the CI
# integration tests load (.github/workflows/integration.yml). CI has no Jira
# distribution to generate data from, so the dump of a synthetic import is
# committed instead: Solstice's own schema, synthetic data only.
#
# What it does: runs the synthetic import (scripts/e2e-import.sh --import-only),
# then dumps the MySQL-sourced result (solstice_e2e_my) with mariadb-dump, in a
# stable form: no dump date, no comments, rows in primary key order, one row per
# INSERT unless that makes the file larger than 2 MB, and the dates the import
# sets itself (not the fixture's) replaced by one fixed value, so that two
# regenerations give the same file. It then checks that the seed is synthetic.
#
# When to run it: after changing the fixture (backend/internal/jirafixture) or
# the migrations. The seed may lag behind the migrations: CI migrates it after
# loading it. Commit the new file with the change that required it.
#
# What it needs: the containers of compose.yaml (docker compose --profile import up -d) and the
# entity model of the newest Jira version (scripts/jira-entitydefs.sh, or
# JIRA_DIST; see e2e-import.sh).
#
#   JIRA_DIST=/path/to/atlassian-jira-software-X-standalone ./scripts/make-test-seed.sh
#   ./scripts/make-test-seed.sh --check    # only check the committed seed (no containers)
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
seed="$root/backend/tests/testdata/seed.sql"
max=$((2 * 1024 * 1024))
# Any valid date will do for the import's own dates.
fixed_date="2026-10-01 18:00:00.000"

# The fixture gives every account an @exemple.fr address and its own name, and
# every URL an example/exemple host; anything else in the seed means real data
# slipped in.
check_synthetic() {
  local other
  other=$(grep -oE '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' "$seed" | grep -viE '@exemple\.fr$' | sort -u || true)
  if [ -n "$other" ]; then
    echo "the seed holds e-mail addresses that are not the fixture's (@exemple.fr), it is not synthetic:" >&2
    echo "$other" | sed 's/^[^@]*/…/' >&2
    return 1
  fi
  other=$(grep -oE 'https?://[A-Za-z0-9.-]+' "$seed" | sed -E 's|^https?://||' | grep -vE '^([a-z0-9-]+\.)*(example|exemple)\.[a-z]+$' | sort -u || true)
  if [ -n "$other" ]; then
    echo "the seed holds URL hosts that are not example.* or exemple.*, it is not synthetic:" >&2
    echo "$other" >&2
    return 1
  fi
  (cd "$root/backend" && go test ./internal/jirafixture -run TestCommittedSeedAccounts -count=1 >&2)
  echo "› the seed only holds the fixture's accounts, @exemple.fr addresses and example hosts"
}

if [ "${1:-}" = "--check" ]; then check_synthetic; exit; fi

"$root/scripts/e2e-import.sh" --import-only

dump() { # mariadb-dump options...
  docker compose -f "$root/compose.yaml" exec -T mariadb mariadb-dump -uroot -proot \
    --single-transaction --skip-comments --skip-dump-date --skip-add-locks --order-by-primary "$@" solstice_e2e_my
}

# The tables whose datetime columns are all set when the import runs (the
# fixture's dates in the other tables are already stable).
normalise() {
  sed -E "/^INSERT INTO \`(boards|filters|schema_migrations|users)\` /s/'[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9:.]+'/'$fixed_date'/g"
}

echo "› dump of solstice_e2e_my"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
dump --skip-extended-insert | normalise > "$tmp"
if [ "$(wc -c < "$tmp")" -gt "$max" ]; then
  echo "  more than 2 MB with one row per INSERT, using extended inserts"
  dump | normalise > "$tmp"
fi

mkdir -p "$(dirname "$seed")"
mv "$tmp" "$seed"
echo "  $(wc -c < "$seed" | tr -d ' ') bytes"
check_synthetic
