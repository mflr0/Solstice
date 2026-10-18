#!/usr/bin/env bash
# End-to-end import test. Generates the same synthetic Jira database on
# PostgreSQL and MySQL, imports both into Solstice, then checks that:
#   1. both imports are identical, table by table;
#   2. counts match what the generator reports;
#   3. invariants hold (sub-tasks, secured issues, copied files, passwords).
#
# With MESSY=1 the generator adds the quirks of old Jira instances (-messy).
# The import must still succeed, both engines must still agree, and what
# cannot be imported must be in the report's warnings. The Go tests on the
# imported database are skipped: they know the standard data, which the quirks
# change. The inactive accounts the importer creates for deleted users are
# added to the expected count.
#
# Needs the containers of compose.yaml (docker compose up -d) and the Jira
# distribution next to the repository (its entitymodel.xml describes the schema).
#
# With --import-only, it stops once both imports are done (make-test-seed.sh
# uses this to dump the result).
set -euo pipefail

case "$*" in
  ""|--import-only) ;;
  *) echo "usage: e2e-import.sh [--import-only]" >&2; exit 2 ;;
esac

root=$(cd "$(dirname "$0")/.." && pwd)
dist=${JIRA_DIST:-$(ls -d "$root"/atlassian-jira-software-*-standalone 2>/dev/null | head -1)}
issues=${ISSUES:-300}
messy=${MESSY:-}
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
    -issues "$issues" -seed "$seed" -tz "$tz" ${messy:+-messy} > "$work/fixture-$engine.json"
  echo "› import ($engine)"
  mariadb -e "DROP DATABASE IF EXISTS solstice_e2e_$engine; CREATE DATABASE solstice_e2e_$engine;
              GRANT ALL ON solstice_e2e_$engine.* TO 'solstice'@'%';"
  SOLSTICE_DB_URL="mysql://solstice:solstice@localhost:13306/solstice_e2e_$engine" SOLSTICE_DATA="$work/data-$engine" \
    "$work/solstice" import -from "$src" -tz "$tz" -jira-home "$work/home-$engine" -json > "$work/report-$engine.json"
done

if [ "${1:-}" = "--import-only" ]; then exit 0; fi

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
                     AND NOT (table_name IN ('filters', 'boards') AND column_name IN ('created_at', 'updated_at'))")
  a=$(mariadb -e "SELECT COUNT(*), COALESCE(SUM(CRC32(CONCAT_WS('|', $cols))), 0) FROM solstice_e2e_pg.$t")
  b=$(mariadb -e "SELECT COUNT(*), COALESCE(SUM(CRC32(CONCAT_WS('|', $cols))), 0) FROM solstice_e2e_my.$t")
  check "$t" "$a" "$b"
done

echo "› nombres conformes au générateur"
expect() { python3 -I -c "import json,sys; print(json.load(open(sys.argv[1]))[sys.argv[2]])" "$work/fixture-pg.json" "$1"; }
expect_messy() { python3 -I -c "import json,sys; print(json.load(open(sys.argv[1]))['messy'][sys.argv[2]])" "$work/fixture-pg.json" "$1"; }
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
check "tableaux" "$(expect boards)" "$(count boards)"
check "colonnes de tableaux" "$(expect board_columns)" "$(count board_columns)"
check "statuts des colonnes" "$(expect board_column_statuses)" "$(count board_column_statuses)"
check "filtres rapides" "$(expect quick_filters)" "$(count board_quick_filters)"
check "couloirs" "$(expect swimlanes)" "$(count board_swimlanes)"
check "sprints" "$(( $(expect sprints_closed) + $(expect sprints_active) + $(expect sprints_future) ))" "$(count sprints)"
check "sprints actifs" "$(expect sprints_active)" "$(count "sprints WHERE state = 'active'")"
check "tickets dans les sprints" "$(expect sprint_memberships)" "$(count sprint_issues)"
check "tickets classés" "$(expect ranked_issues)" "$(count 'issues WHERE rank_key IS NOT NULL')"
check "tickets rattachés à une épopée" "$(expect epic_links)" "$(count 'issues WHERE epic_id IS NOT NULL')"
ghosts=0
if [ -n "$messy" ]; then ghosts=$(expect_messy ghost_users); fi
check "comptes inactifs (désactivés, supprimés, sans accès applicatif)" \
  "$(( $(expect inactive_users) + $(expect deleted_users) + $(expect users_without_access) + ghosts ))" \
  "$(count 'users WHERE NOT active')"

if [ -n "$messy" ]; then
  echo "› avertissements de l'import (données désordonnées)"
  warned() { # description, expected warning: the report holds exactly this line
    if python3 -I -c "import json,sys; sys.exit(0 if sys.argv[2] in json.load(open(sys.argv[1]))['warnings'] else 1)" \
      "$work/report-pg.json" "$2"; then echo "  ok    $1"; else echo "  ÉCHEC $1 : \"$2\" absent"; fail=1; fi
  }
  fields() { python3 -I -c "import json,sys; print(', '.join(map(str, json.load(open(sys.argv[1]))['messy']['misfit_field_ids'])))" "$work/fixture-pg.json"; }
  many() { if [ "$1" = 1 ]; then echo "1 $2"; else echo "$1 $3"; fi; } # count, singular, plural
  check "mêmes avertissements sur PostgreSQL et MySQL" \
    "$(python3 -I -c "import json,sys; print(json.load(open(sys.argv[1]))['warnings'])" "$work/report-pg.json")" \
    "$(python3 -I -c "import json,sys; print(json.load(open(sys.argv[1]))['warnings'])" "$work/report-my.json")"
  warned "utilisateurs inconnus" "$(many "$ghosts" user users) named by issues, comments or history but unknown to Jira, kept as inactive accounts"
  warned "détails d'historique orphelins" \
    "$(many "$(expect_messy orphan_change_items)" "change detail" "change details") of a missing change, not imported"
  warned "temps saisis orphelins" "$(many "$(expect_messy orphan_worklogs)" "work log" "work logs") of a missing issue, not imported"
  warned "commentaires orphelins" "$(many "$(expect_messy orphan_comments)" comment comments) of a missing issue, not imported"
  warned "liens orphelins" \
    "$(many "$(expect_messy orphan_issue_links)" "issue link" "issue links") of a missing issue or link type, not imported"
  warned "composants manquants" \
    "$(many "$(expect_messy orphan_component_links)" "component link" "component links") to a missing component, not imported"
  warned "anciennes clés de tickets manquants" \
    "$(many "$(expect_messy orphan_moved_keys)" "old issue key" "old issue keys") of a missing issue, not imported"
  warned "anciennes clés données à deux tickets" \
    "$(many "$(expect_messy duplicate_keys)" "old issue key" "old issue keys") already given to another issue (it keeps the first), not imported"
  warned "observateurs et votes d'un utilisateur inconnu" \
    "$(many "$(expect_messy ghost_watches)" "watch or vote" "watches or votes") of an unknown user or a missing issue, not imported"
  warned "liens externes sans adresse" \
    "$(many "$(expect_messy empty_remote_links)" "remote link" "remote links") without a URL, not imported"
  warned "valeurs d'un ancien type de champ" \
    "$(many "$(expect_messy misfit_field_values)" value values) stored for a former type of their custom field, not imported (fields $(fields))"
  echo "  avertissements du rapport :"
  python3 -I -c "import json,sys; [print('    -', w) for w in json.load(open(sys.argv[1]))['warnings']]" "$work/report-pg.json"
fi

if [ -n "$messy" ]; then
  echo "› tests Go ignorés (ils connaissent les données standard)"
else
  echo "› tests Go (unitaires, et API sur la base importée)"
  # An empty database for the setup wizard's tests, which fill it.
  mariadb -e "DROP DATABASE IF EXISTS solstice_e2e_setup;
              CREATE DATABASE solstice_e2e_setup CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
              GRANT ALL ON solstice_e2e_setup.* TO 'solstice'@'%';"
  if (cd "$root/backend" && SOLSTICE_TEST_DB=mysql://solstice:solstice@localhost:13306/solstice_e2e_pg \
      SOLSTICE_TEST_EMPTY_DB=mysql://solstice:solstice@localhost:13306/solstice_e2e_setup go test ./... -count=1); then
    echo "  ok    go test"
  else
    echo "  ÉCHEC go test"; fail=1
  fi
fi

if [ $fail = 0 ]; then echo "Tout est conforme."; else echo "Des écarts ont été trouvés."; exit 1; fi
