#!/usr/bin/env bash
# End-to-end import test. Generates the same synthetic Jira database on
# PostgreSQL and MySQL, imports both into Solstice, then checks that:
#   1. both imports are identical, table by table;
#   2. counts match what the generator reports;
#   3. invariants hold (sub-tasks, secured issues, copied files, passwords).
#
# Needs the containers of compose.yaml (docker compose up -d) and the Jira
# distribution next to the repository (its entitymodel.xml describes the schema).
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
dist=${JIRA_DIST:-$(ls -d "$root"/atlassian-jira-software-*-standalone 2>/dev/null | head -1)}
issues=${ISSUES:-300}
seed=${SEED:-42}
tz=Europe/Paris
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

mariadb() { docker compose -f "$root/compose.yaml" exec -T mariadb mariadb -uroot -proot -N "$@"; }

echo "› compilation"
(cd "$root/backend" && go build -o "$work/solstice" ./cmd/solstice && go build -o "$work/jira-fixture" ./cmd/jira-fixture)

for engine in pg my; do
  case $engine in
    pg) src=postgres://jira:jira@localhost:15432/jira ;;
    my) src=mysql://jira:jira@localhost:23306/jira ;;
  esac
  echo "› base Jira synthétique ($engine, $issues tickets)"
  "$work/jira-fixture" -url "$src" -jira-dist "$dist" -jira-home "$work/home-$engine" \
    -issues "$issues" -seed "$seed" -tz "$tz" > "$work/fixture-$engine.json"
  echo "› import ($engine)"
  mariadb -e "DROP DATABASE IF EXISTS solstice_e2e_$engine; CREATE DATABASE solstice_e2e_$engine;
              GRANT ALL ON solstice_e2e_$engine.* TO 'solstice'@'%';"
  "$work/solstice" import -db "mysql://solstice:solstice@localhost:13306/solstice_e2e_$engine" -data "$work/data-$engine" \
    -from "$src" -tz "$tz" -jira-home "$work/home-$engine" -json > "$work/report-$engine.json"
done

fail=0
check() { # description, expected, actual
  if [ "$2" = "$3" ]; then echo "  ok    $1 ($3)"; else echo "  ÉCHEC $1 : attendu $2, obtenu $3"; fail=1; fi
}

echo "› imports PostgreSQL et MySQL identiques"
tables=$(mariadb -e "SELECT table_name FROM information_schema.tables WHERE table_schema = 'solstice_e2e_pg'
                     AND table_name NOT IN ('schema_migrations', 'sessions') ORDER BY table_name")
for t in $tables; do
  # Dates set at import time (Jira does not store them) differ by nature;
  # everything else must match.
  cols=$(mariadb -e "SELECT GROUP_CONCAT(column_name ORDER BY ordinal_position) FROM information_schema.columns
                     WHERE table_schema = 'solstice_e2e_pg' AND table_name = '$t'
                     AND NOT (table_name = 'users' AND column_name = 'created_at')
                     AND NOT (table_name = 'filters' AND column_name IN ('created_at', 'updated_at'))")
  a=$(mariadb -e "SELECT COUNT(*), COALESCE(SUM(CRC32(CONCAT_WS('|', $cols))), 0) FROM solstice_e2e_pg.$t")
  b=$(mariadb -e "SELECT COUNT(*), COALESCE(SUM(CRC32(CONCAT_WS('|', $cols))), 0) FROM solstice_e2e_my.$t")
  check "$t" "$a" "$b"
done

echo "› nombres conformes au générateur"
expect() { python3 -I -c "import json,sys; print(json.load(open(sys.argv[1]))[sys.argv[2]])" "$work/fixture-pg.json" "$1"; }
count() { mariadb -e "SELECT COUNT(*) FROM solstice_e2e_pg.$1"; }
check "tickets" "$(expect issues)" "$(count issues)"
check "sous-tâches" "$(expect subtasks)" "$(count 'issues WHERE parent_id IS NOT NULL')"
check "tickets confidentiels" "$(expect secured_issues)" "$(count 'issues WHERE security_level_id IS NOT NULL')"
check "commentaires" "$(expect comments)" "$(count comments)"
check "commentaires restreints" "$(expect restricted_comments)" \
  "$(count 'comments WHERE visibility_group IS NOT NULL OR visibility_role_id IS NOT NULL')"
check "temps saisis" "$(expect worklogs)" "$(count worklogs)"
check "pièces jointes" "$(expect attachments)" "$(count attachments)"
check "fichiers manquants" "$(expect attachment_files_missing)" "$(count 'attachments WHERE file_missing')"
check "fichiers copiés" "$(( $(expect attachments) - $(expect attachment_files_missing) ))" \
  "$(find "$work/data-pg/attachments" -type f | wc -l | tr -d ' ')"
check "liens" "$(expect links_non_subtask)" "$(count issue_links)"
check "liens externes" "$(expect remote_links)" "$(count remote_links)"
check "observateurs" "$(expect watchers)" "$(count watchers)"
check "votes" "$(expect votes)" "$(count votes)"
check "étiquettes" "$(( $(expect labels_system) + $(expect labels_custom) ))" "$(count labels)"
check "valeurs de champs" "$(expect custom_field_values)" "$(count custom_field_values)"
check "historique" "$(expect change_groups)" "$(count change_groups)"
check "détails d'historique" "$(expect change_items)" "$(count change_items)"
check "anciennes clés de tickets" "$(expect moved_issue_keys)" "$(count moved_issue_keys)"
check "mots de passe Jira repris" "$(expect users_with_pkcs5s2)" \
  "$(count "users WHERE password_hash LIKE '{PKCS5S2}%'")"
check "filtres enregistrés" "$(expect filters)" "$(count filters)"
check "partages de filtres" "$(expect filter_shares)" "$(count filter_shares)"
check "filtres favoris" "$(expect filter_favourites)" "$(count filter_favourites)"
check "comptes inactifs (désactivés, supprimés, sans accès applicatif)" \
  "$(( $(expect inactive_users) + $(expect deleted_users) + $(expect users_without_access) ))" \
  "$(count 'users WHERE NOT active')"

echo "› tests Go (unitaires, et API sur la base importée)"
if (cd "$root/backend" && SOLSTICE_TEST_DB=mysql://solstice:solstice@localhost:13306/solstice_e2e_pg go test ./... -count=1); then
  echo "  ok    go test"
else
  echo "  ÉCHEC go test"; fail=1
fi

if [ $fail = 0 ]; then echo "Tout est conforme."; else echo "Des écarts ont été trouvés."; exit 1; fi
