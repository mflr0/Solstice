#!/bin/bash
# Picks, claims and finishes Solstice tickets for the solstice-issue skill.
#
#   pick.sh [number] [label...]     list candidates (or check one ticket)
#   pick.sh --claim N SESSION        claim N; exit 3 if another session won
#   pick.sh --release N              drop a claim
#   pick.sh --done N PR              after merge: unlabel, drop claim comments,
#                                    close, tick tracker
set -euo pipefail

R=mflr0/Solstice
LABEL='in progress'
STALE_H=24

die() { echo "$*" >&2; exit 1; }

# Epoch seconds of an ISO-8601 UTC time.
epoch() { date -j -u -f '%Y-%m-%dT%H:%M:%SZ' "$1" +%s; }

# Time the label was last added to issue $1, empty if never.
labeled_at() {
  gh api "repos/$R/issues/$1/events?per_page=100" \
    --jq "[.[] | select(.event == \"labeled\" and .label.name == \"$LABEL\")] | last | .created_at // empty"
}

# Whether the claim on $1 is stale: label older than STALE_H and no open PR
# naming the issue got a commit within STALE_H.
stale() {
  local at last now
  at=$(labeled_at "$1")
  [ -n "$at" ] || return 0
  now=$(date -u +%s)
  [ $((now - $(epoch "$at"))) -gt $((STALE_H * 3600)) ] || return 1
  last=$(gh pr list -R "$R" --state open --search "$1" --json commits \
    --jq '[.[].commits[-1].committedDate] | max // empty')
  [ -z "$last" ] && return 0
  [ $((now - $(epoch "$last"))) -gt $((STALE_H * 3600)) ]
}

claim() {
  local n=$1 session=$2 has id at winner
  has=$(gh issue view "$n" -R "$R" --json state,labels \
    --jq "[.state, (.labels | map(.name) | index(\"$LABEL\") != null)] | @tsv")
  case $has in
    OPEN*true)
      if stale "$n"; then
        echo "#$n: stale claim, releasing it"
        gh issue edit "$n" -R "$R" --remove-label "$LABEL" >/dev/null
      else
        echo "#$n: already claimed"; exit 3
      fi ;;
    OPEN*) ;;
    *) die "#$n is not open" ;;
  esac
  gh label create "$LABEL" -R "$R" --color FBCA04 \
    --description 'A Claude session is working on it' >/dev/null 2>&1 || true
  gh issue edit "$n" -R "$R" --add-label "$LABEL" >/dev/null
  id=$(gh api "repos/$R/issues/$n/comments" -f body="Claimed by session $session." --jq .id)
  # Every session uses the same GitHub account, so the label's actor cannot
  # break a race: the first claim comment since the label was set does.
  at=$(labeled_at "$n")
  winner=$(gh api "repos/$R/issues/$n/comments?per_page=100" --jq \
    "[.[] | select(.created_at >= \"$at\") | .body | capture(\"^Claimed by session (?<s>\\\\S+)\\\\.\$\") | .s] | first // empty")
  if [ "$winner" != "$session" ]; then
    gh api -X DELETE "repos/$R/issues/comments/$id" >/dev/null
    echo "#$n: claimed first by $winner"; exit 3
  fi
  echo "#$n: claimed by $session"
}

release() {
  gh issue edit "$1" -R "$R" --remove-label "$LABEL" >/dev/null
  echo "#$1: released"
}

done_() {
  local n=$1 pr=$2 tracker body new id
  gh issue edit "$n" -R "$R" --remove-label "$LABEL" >/dev/null 2>&1 || true
  for id in $(gh api "repos/$R/issues/$n/comments?per_page=100" --jq \
    '.[] | select(.body | test("^Claimed by session \\S+\\.$")) | .id'); do
    gh api -X DELETE "repos/$R/issues/comments/$id" >/dev/null
  done
  echo "#$n: claim comments deleted"
  if [ "$(gh issue view "$n" -R "$R" --json state --jq .state)" = OPEN ]; then
    gh issue close "$n" -R "$R" -c "Done in #$pr." >/dev/null
  fi
  echo "#$n: closed"
  tracker=$(gh issue view "$n" -R "$R" --json body \
    --jq '[.body | capture("Tracked in #(?<t>[0-9]+)")][0].t // empty')
  [ -n "$tracker" ] || return 0
  body=$(gh issue view "$tracker" -R "$R" --json body --jq .body)
  new=$(printf '%s\n' "$body" | sed -E "s/^([[:space:]]*)- \[ \] #$n([^0-9]|\$)/\1- [x] #$n\2/")
  if [ "$new" != "$body" ]; then
    printf '%s\n' "$new" | gh issue edit "$tracker" -R "$R" --body-file - >/dev/null
    echo "#$tracker: ticked #$n"
  fi
}

list() {
  local num=null filters=() all cands deps q states claimed n
  for a in "$@"; do
    case $a in
      *[!0-9]*) filters+=("$a") ;;
      *) num=$a ;;
    esac
  done
  all=$(gh issue list -R "$R" --state open --limit 500 \
    --json number,title,labels,assignees,body)
  cands=$(jq -c --argjson num "$num" \
    --argjson filters "$(printf '%s\n' ${filters[@]+"${filters[@]}"} | jq -R . | jq -sc 'map(select(. != ""))')" \
    --arg label "$LABEL" '
    def nums: [scan("#([0-9]+)") | .[0] | tonumber];
    def waits: [scan("(?i)waits for[^\n]*") | nums[]];
    def tracker: [capture("Tracked in #(?<t>[0-9]+)")][0].t // null | if . then tonumber else null end;
    def scope: [capture("In scope: (?<f>[^\n]*)")][0].f // "" | rtrimstr(".") | split(", ") | map(select(. != ""));
    def isbug: .labels | map(.name) | index("bug") != null;
    (map({key: (.number | tostring), value: .body}) | from_entries) as $bodies
    | ([.[] | .body | tracker | select(.)] | unique) as $trackers
    | [.[] | select(isbug) | {number, files: (.body | scope)}] as $bugs
    | map(
        .number as $n
        | (.body | tracker) as $tr
        | (($bodies[$tr | tostring] // "") | split("\n")) as $lines
        | ($lines | map(test("^\\s*- \\[.\\] #\($n)\\b")) | index(true)) as $pos
        | (.body | scope) as $files
        | {
            number, title,
            labels: (.labels | map(.name)),
            assigned: (.assignees | length > 0),
            tracker: $tr,
            pos: $pos,
            waits: ((.body | waits) + (if $pos then $lines[$pos] | waits else [] end) | unique),
            # Open bugs go before other tickets on their files. Bugs are not
            # checked against each other: two on one file would wait forever.
            shared: (if isbug then [] else
              [$bugs[] | select(.number != $n and any(.files[]; . as $f | $files | index($f))) | .number]
              end),
          }
        | select($trackers | index($n) | not)
      )
    | map(select(
        if $num != null then .number == $num
        else (.assigned | not)
          and (.labels as $ls | all($filters[]; . as $f | $ls | index($f)))
          and (($filters | index("enhancement")) or (.labels | index("enhancement") | not))
        end))
    | sort_by([.tracker == null, .tracker // 0, .pos // 1e9, .number])
    | map(.claimed = (.labels | index($label) != null))
  ' <<<"$all")
  [ "$cands" != "[]" ] || { echo "No candidate."; return; }

  deps=$(jq -r '[.[].waits[]] | unique | .[]' <<<"$cands")
  states='{}'
  if [ -n "$deps" ]; then
    q='query { repository(owner: "mflr0", name: "Solstice") {'
    for d in $deps; do
      q+=" n$d: issueOrPullRequest(number: $d) { __typename ... on Issue { state } ... on PullRequest { state merged } }"
    done
    q+=' } }'
    states=$(gh api graphql -f query="$q" --jq '.data.repository' 2>/dev/null || echo '{}')
  fi

  claimed=''
  for n in $(jq -r '.[] | select(.claimed) | .number' <<<"$cands"); do
    if stale "$n"; then claimed+="$n "; fi
  done

  jq -r --argjson st "$states" --arg stale "$claimed" --argjson num "$num" '
    def why($d): ($st["n\($d)"] // null) as $s
      | if $s == null then null
        elif $s.__typename == "Issue" then (if $s.state == "OPEN" then "#\($d) open" else null end)
        elif $s.merged then null
        else "#\($d) PR \($s.state | ascii_downcase)" end;
    ($stale | split(" ") | map(select(. != "") | tonumber)) as $stl
    | map(. + {blockers: ([.waits[] as $d | why($d) | select(.)]
                 + if .shared == [] then [] else ["bugs on its files: \(.shared | map("#\(.)") | join(" "))"] end),
               stale: (.number as $n | $stl | index($n) != null)})
    | (map(select((.blockers | length == 0) and ((.claimed | not) or .stale)))) as $free
    | (map(select(.claimed and (.stale | not)))) as $taken
    | (map(select((.blockers | length > 0) and ((.claimed | not) or .stale)))) as $blocked
    | if $free == [] and $num == null then
        "Nothing free: \($blocked | length) blocked, \($taken | length) claimed."
      else
        "Candidates, in order (\($free | length)):",
        ($free[:10][] | "  #\(.number) [\(.labels | join(","))] \(.title)\(if .stale then " (stale claim)" else "" end)"),
        (if ($free | length) > 10 then "  +\(($free | length) - 10) more" else empty end),
        (if ($taken | length) > 0 then "Claimed: \($taken | map("#\(.number)") | join(" "))" else empty end),
        (if ($blocked | length) > 0 then "Blocked (\($blocked | length)):",
          ($blocked[:10][] | "  #\(.number) by \(.blockers | join(", "))") else empty end),
        (if ($blocked | length) > 10 then "  +\(($blocked | length) - 10) more" else empty end)
      end
  ' <<<"$cands"
}

case ${1:-} in
  --claim) [ $# -eq 3 ] || die "usage: pick.sh --claim N SESSION"; claim "$2" "$3" ;;
  --release) [ $# -eq 2 ] || die "usage: pick.sh --release N"; release "$2" ;;
  --done) [ $# -eq 3 ] || die "usage: pick.sh --done N PR"; done_ "$2" "$3" ;;
  -h|--help) sed -n '2,8p' "$0" ;;
  *) list "$@" ;;
esac
