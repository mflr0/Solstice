#!/usr/bin/env bash
# Downloads the last long-term support release of each major Jira version and
# keeps only its entity model (atlassian-jira/WEB-INF/classes/entitydefs), the
# part jira-fixture reads. Each release lands in
# $JIRA_ENTITYDEFS/atlassian-jira-software-<version>-standalone, by default
# next to the repository. Releases already there are not downloaded again.
#
# Atlassian's files stay out of the repository: this cache is not committed.
set -euo pipefail

versions=(7.13.18 8.20.30 9.12.25 10.3.12 11.3.12)

root=$(cd "$(dirname "$0")/.." && pwd)
cache=${JIRA_ENTITYDEFS:-$(dirname "$root")/jira-entitydefs}
defs=atlassian-jira/WEB-INF/classes/entitydefs
mkdir -p "$cache"
tmp=
trap 'if [ -n "$tmp" ]; then rm -rf "$tmp"; fi' EXIT

for v in "${versions[@]}"; do
  dist=$cache/atlassian-jira-software-$v-standalone
  if [ -f "$dist/$defs/entitymodel.xml" ]; then
    echo "› $v: already in $dist"
    continue
  fi
  echo "› $v: download"
  tmp=$(mktemp -d "$cache/.partial.XXXXXX")
  # The archive is streamed: only the entity model reaches the disk.
  curl -fsSL "https://product-downloads.atlassian.com/software/jira/downloads/atlassian-jira-software-$v.tar.gz" \
    | tar -xz -C "$tmp" --strip-components 1 "*/$defs/*"
  rm -rf "$dist"
  mv "$tmp" "$dist"
  tmp=
done
