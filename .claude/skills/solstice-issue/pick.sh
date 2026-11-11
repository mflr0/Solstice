#!/bin/bash
# Git and pull-request steps of the solstice-issue skill. The tickets live in
# Solstice (SOL) and the skill handles them through its MCP tools.
#
#   pick.sh --start TOPIC             create the worktree and its branch
#   pick.sh --ship KEY TOPIC draft|ready [--title T] [--note T]
#                                     push, open the PR, comment "Closes KEY."
#   pick.sh --finish PR TOPIC         wait for CI, merge, remove the worktree
#                                     and branches, pull the base; exit 4 if a
#                                     check failed
#   pick.sh --last-commit KEY         time of the last commit of the open PRs
#                                     naming KEY, empty if none
set -euo pipefail

R=mflr0/Solstice
BASE=feat/v1.0

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

start() {
  local topic=$1 main wt br=v1.0/$1
  main=$(main_checkout)
  wt=$(worktree_of "$main" "$topic")
  [ -n "$(git -C "$main" config --local user.email)" ] ||
    die "no local git identity in $main: ask the user, the global one is not for this repo"
  [ ! -e "$wt" ] || die "$wt already exists"
  ! git -C "$main" show-ref --verify --quiet "refs/heads/$br" || die "branch $br already exists"
  git -C "$main" fetch -q origin
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
    *) die "usage: pick.sh --ship KEY TOPIC draft|ready [--title T] [--note T]" ;;
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
  if ! gh api "repos/$R/issues/$pr/comments?per_page=100" --jq '.[].body' | grep -q "^Closes $n\.\$"; then
    body="Closes $n."
    [ -z "$note" ] || body+=$'\n\n'"$note"
    gh pr comment "$pr" -R "$R" --body "$body" >/dev/null
  fi
  echo "PR #$pr ($mode): https://github.com/$R/pull/$pr"
}

finish() {
  local pr=$1 topic=$2 main wt br=v1.0/$2 state draft head_ref head i failed
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
  if [ -d "$wt" ]; then git -C "$main" worktree remove "$wt"; fi
  if git -C "$main" show-ref --verify --quiet "refs/heads/$br"; then git -C "$main" branch -q -D "$br"; fi
  if [ -n "$(git -C "$main" ls-remote --heads origin "$br")" ]; then git -C "$main" push -q origin --delete "$br"; fi
  git -C "$main" checkout -q "$BASE"
  git -C "$main" pull -q --ff-only
  echo "removed: worktree $wt, branch $br (local and remote)"
  echo "$main: $BASE at $(git -C "$main" log -1 --format='%h %s')"
}

last_commit() {
  gh pr list -R "$R" --state open --search "\"Closes $1.\" in:comments" --json commits \
    --jq '[.[].commits[-1].committedDate] | max // empty'
}

case ${1:-} in
  --start) [ $# -eq 2 ] || die "usage: pick.sh --start TOPIC"; start "$2" ;;
  --ship) [ $# -ge 4 ] || die "usage: pick.sh --ship KEY TOPIC draft|ready [--title T] [--note T]"; shift; ship "$@" ;;
  --finish) [ $# -eq 3 ] || die "usage: pick.sh --finish PR TOPIC"; finish "$2" "$3" ;;
  --last-commit) [ $# -eq 2 ] || die "usage: pick.sh --last-commit KEY"; last_commit "$2" ;;
  -h|--help) sed -n '2,13p' "$0" ;;
  *) die "usage: see pick.sh --help" ;;
esac
