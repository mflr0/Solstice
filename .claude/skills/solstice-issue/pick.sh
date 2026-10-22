#!/bin/bash
# Picks, claims and finishes Solstice tickets for the solstice-issue skill.
#
#   pick.sh [number] [label...]      list candidates (or check one ticket),
#                                     then print the first free one in full
#   pick.sh --start N SESSION TOPIC   claim N and create its worktree; exit 3
#                                     if another session won
#   pick.sh --claim N SESSION         claim N only; exit 3 if another session won
#   pick.sh --release N               drop a claim
#   pick.sh --ship N TOPIC draft|ready [--title T] [--note T]
#                                     push, open the PR, comment "Closes #N."
#   pick.sh --finish N PR TOPIC       wait for CI, merge, run --done, remove the
#                                     worktree and branches, pull the base;
#                                     exit 4 if a check failed
#   pick.sh --done N PR               after merge: unlabel, drop claim comments,
#                                     close, tick tracker
set -euo pipefail

R=mflr0/Solstice
BASE=feat/v1.0
LABEL='in progress'
STALE_H=24

die() { echo "$*" >&2; exit 1; }

# The main checkout, found from the working directory or this script.
main_checkout() {
  local d
  d=$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null) ||
    d=$(git -C "$(dirname "$0")" rev-parse --path-format=absolute --git-common-dir 2>/dev/null) ||
    die "run pick.sh from inside the Solstice repository"
  dirname "$d"
}

worktree_of() { echo "$(dirname "$1")/$(basename "$1")-$2"; }

# Epoch seconds of an ISO-8601 UTC time, with GNU date or else BSD date.
epoch() { date -u -d "$1" +%s 2>/dev/null || date -j -u -f '%Y-%m-%dT%H:%M:%SZ' "$1" +%s; }

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
    "[.[] | select(.created_at >= \"$at\") | .body | capture(\"^Claimed by session (?<s>.+)\\\\.\$\") | .s] | first // empty")
  if [ "$winner" != "$session" ]; then
    gh api -X DELETE "repos/$R/issues/comments/$id" >/dev/null
    echo "#$n: claimed first by $winner"; exit 3
  fi
  echo "#$n: claimed by $session"
}

start() {
  local n=$1 session=$2 topic=$3 main wt br=v1.0/$3
  main=$(main_checkout)
  wt=$(worktree_of "$main" "$topic")
  [ -n "$(git -C "$main" config --local user.email)" ] ||
    die "no local git identity in $main: ask the user, the global one is not for this repo"
  [ ! -e "$wt" ] || die "$wt already exists"
  ! git -C "$main" show-ref --verify --quiet "refs/heads/$br" || die "branch $br already exists"
  git -C "$main" fetch -q origin
  claim "$n" "$session"
  git -C "$main" worktree add -q -b "$br" "$wt" "origin/$BASE"
  echo "worktree: $wt (branch $br from origin/$BASE)"
}

ship() {
  local n=$1 topic=$2 mode=$3 title='' note='' main wt br=v1.0/$2 count pr url body
  local draft=()
  shift 3
  while [ $# -gt 0 ]; do
    case $1 in
      --title) [ $# -ge 2 ] || die "--title needs a value"; title=$2; shift 2 ;;
      --note) [ $# -ge 2 ] || die "--note needs a value"; note=$2; shift 2 ;;
      *) die "unknown argument: $1" ;;
    esac
  done
  case $mode in
    draft) draft=(--draft) ;;
    ready) ;;
    *) die "usage: pick.sh --ship N TOPIC draft|ready [--title T] [--note T]" ;;
  esac
  main=$(main_checkout)
  wt=$(worktree_of "$main" "$topic")
  [ -d "$wt" ] || die "no worktree at $wt"
  [ -z "$(git -C "$wt" status --porcelain)" ] || die "$wt has uncommitted changes"
  git -C "$wt" fetch -q origin
  count=$(git -C "$wt" rev-list --count "origin/$BASE..HEAD")
  [ "$count" -gt 0 ] || die "no commit on $br"
  if [ -z "$title" ]; then
    [ "$count" -eq 1 ] || die "$count commits: pass --title"
    title=$(git -C "$wt" log -1 --format=%s)
  fi
  git -C "$wt" push -q -u origin "$br"
  pr=$(gh pr list -R "$R" --head "$br" --state open --json number --jq '.[0].number // empty')
  if [ -z "$pr" ]; then
    url=$(gh pr create -R "$R" --base "$BASE" --head "$br" --title "$title" --body "" ${draft[@]+"${draft[@]}"})
    pr=${url##*/}
  fi
  # The stale-claim check finds the pull request through this comment.
  if ! gh api "repos/$R/issues/$pr/comments?per_page=100" --jq '.[].body' | grep -q "^Closes #$n\.\$"; then
    body="Closes #$n."
    [ -z "$note" ] || body+=$'\n\n'"$note"
    gh pr comment "$pr" -R "$R" --body "$body" >/dev/null
  fi
  echo "PR #$pr ($mode): https://github.com/$R/pull/$pr"
}

finish() {
  local n=$1 pr=$2 topic=$3 main wt br=v1.0/$3 state draft head_ref head i failed
  main=$(main_checkout)
  wt=$(worktree_of "$main" "$topic")
  state=$(gh pr view "$pr" -R "$R" --json state,isDraft,headRefName,headRefOid \
    --jq '"\(.state) \(.isDraft) \(.headRefName) \(.headRefOid)"')
  read -r state draft head_ref head <<<"$state"
  [ "$head_ref" = "$br" ] || die "PR #$pr is on $head_ref, not $br"
  case $state in
    OPEN)
      [ "$draft" = false ] || gh pr ready "$pr" -R "$R" >/dev/null
      # Checks show up a few seconds after a push.
      for i in $(seq 30); do
        [ "$(gh pr checks "$pr" -R "$R" --json name --jq length 2>/dev/null || echo 0)" -gt 0 ] && break
        sleep 10
      done
      gh pr checks "$pr" -R "$R" --watch --interval 20 >/dev/null 2>&1 || true
      failed=$(gh pr checks "$pr" -R "$R" --json name,bucket,link \
        --jq '.[] | select(.bucket == "fail" or .bucket == "cancel" or .bucket == "pending") | "\(.bucket) \(.name) \(.link)"')
      if [ -n "$failed" ]; then
        printf 'PR #%s not merged:\n%s\n' "$pr" "$failed" >&2
        exit 4
      fi
      # Merge only the commit the checks ran on.
      gh pr merge "$pr" -R "$R" --merge --match-head-commit "$head" >/dev/null
      echo "PR #$pr: merged"
      ;;
    MERGED) echo "PR #$pr: already merged" ;;
    *) die "PR #$pr is $state" ;;
  esac
  done_ "$n" "$pr"
  if [ -d "$wt" ]; then git -C "$main" worktree remove "$wt"; fi
  if git -C "$main" show-ref --verify --quiet "refs/heads/$br"; then git -C "$main" branch -q -D "$br"; fi
  if [ -n "$(git -C "$main" ls-remote --heads origin "$br")" ]; then git -C "$main" push -q origin --delete "$br"; fi
  git -C "$main" checkout -q "$BASE"
  git -C "$main" pull -q --ff-only
  echo "removed: worktree $wt, branch $br (local and remote)"
  echo "$main: $BASE at $(git -C "$main" log -1 --format='%h %s')"
}

release() {
  gh issue edit "$1" -R "$R" --remove-label "$LABEL" >/dev/null
  echo "#$1: released"
}

done_() {
  local n=$1 pr=$2 tracker body new id
  gh issue edit "$n" -R "$R" --remove-label "$LABEL" >/dev/null 2>&1 || true
  for id in $(gh api "repos/$R/issues/$n/comments?per_page=100" --jq \
    '.[] | select(.body | test("^Claimed by session .+\\.$")) | .id'); do
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
  local num=null filters=() all cands deps q states claimed n groups first
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

  groups=$(jq -c --argjson st "$states" --arg stale "$claimed" '
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
    | {$free, $taken, $blocked}
  ' <<<"$cands")

  jq -r --argjson num "$num" '
    .free as $free | .taken as $taken | .blocked as $blocked
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
  ' <<<"$groups"

  first=$(jq -r '.free[0].number // empty' <<<"$groups")
  [ -n "$first" ] || return 0
  echo
  gh issue view "$first" -R "$R" --json number,title,body,comments --jq '
    "=== #\(.number) \(.title)\n\n\(.body)"
    + (.comments | map(select(.body | test("^Claimed by session .+\\.$") | not)
        | "\n\n--- \(.author.login), \(.createdAt[:10]):\n\(.body)") | join(""))'
}

case ${1:-} in
  --start) [ $# -eq 4 ] || die "usage: pick.sh --start N SESSION TOPIC"; start "$2" "$3" "$4" ;;
  --claim) [ $# -eq 3 ] || die "usage: pick.sh --claim N SESSION"; claim "$2" "$3" ;;
  --release) [ $# -eq 2 ] || die "usage: pick.sh --release N"; release "$2" ;;
  --ship) [ $# -ge 4 ] || die "usage: pick.sh --ship N TOPIC draft|ready [--title T] [--note T]"; shift; ship "$@" ;;
  --finish) [ $# -eq 4 ] || die "usage: pick.sh --finish N PR TOPIC"; finish "$2" "$3" "$4" ;;
  --done) [ $# -eq 3 ] || die "usage: pick.sh --done N PR"; done_ "$2" "$3" ;;
  -h|--help) sed -n '2,16p' "$0" ;;
  *) list "$@" ;;
esac
