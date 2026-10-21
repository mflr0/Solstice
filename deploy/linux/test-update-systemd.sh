#!/bin/sh
# Runs the update helper for real: Debian with systemd in a container,
# Solstice 1.0.0 installed by install.sh, then three updates asked for the way
# the web process asks (a request file written as the solstice user):
#   1.0.1 is installed and healthy;
#   1.0.2 has a wrong checksum in its (signed) SHA256SUMS: refused, nothing changes;
#   1.0.3 does not start: rolled back to 1.0.1.
# Releases come from a loopback server through the helper's test mode
# (/usr/local/lib/solstice/update-test), signed with a throwaway key that the
# copy of update.sh in the container trusts. install.sh --with-mariadb sets up
# the database, so the helper also backs it up. The repository is not changed.
#
#   sh deploy/linux/test-update-systemd.sh
#
# Needs Docker (the container is privileged: systemd runs as its PID 1) and
# Go. Takes a few minutes. Exits 0 when every check passes.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
image=solstice-update-test:local
base_image=debian:13-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad
port=8099

host() {
  case $(docker info --format '{{.Architecture}}') in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *)
      echo "unsupported Docker architecture"
      exit 1
      ;;
  esac
  tmp=$(mktemp -d)
  name=solstice-update-test-$$
  trap 'docker rm -f "$name" >/dev/null 2>&1 || true; rm -rf "$tmp"' EXIT INT TERM

  echo "› binaries 1.0.0 and 1.0.1 (linux/$arch)"
  for v in 1.0.0 1.0.1; do
    (cd "$here/../../backend" && CGO_ENABLED=0 GOOS=linux GOARCH=$arch go build -trimpath \
      -ldflags "-s -w -X main.version=$v" -o "$tmp/solstice-$v" ./cmd/solstice)
  done

  echo "› image"
  docker build -q -t "$image" - >/dev/null <<EOF
FROM $base_image
RUN apt-get update \\
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \\
      systemd systemd-sysv dbus curl openssl ca-certificates busybox mariadb-server \\
 && rm -rf /var/lib/apt/lists/*
ENV container=docker
STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]
EOF

  docker run -d --name "$name" --privileged --cgroupns=private --tmpfs /run --tmpfs /run/lock \
    -v "$here:/src/deploy:ro" -v "$tmp:/src/bin:ro" "$image" >/dev/null
  i=0
  until state=$(docker exec "$name" systemctl is-system-running 2>/dev/null) ||
    [ "$state" = degraded ]; do
    i=$((i + 1))
    if [ "$i" -gt 60 ]; then
      echo "systemd did not start in the container (state: $state)"
      docker logs "$name" 2>&1 | tail -n 30
      exit 1
    fi
    sleep 1
  done
  docker exec "$name" sh /src/deploy/test-update-systemd.sh --inside "$arch"
}

pass() { printf 'ok   %s\n' "$1"; }
fail() {
  printf 'FAIL %s\n' "$1"
  failures=$((failures + 1))
}
check() {
  name=$1
  shift
  if "$@"; then pass "$name"; else fail "$name"; fi
}

# release VERSION BINARY [WRONG]: an archive as scripts/release.sh packs it,
# with SHA256SUMS and its signature; WRONG writes a checksum that does not
# match the archive.
release() {
  v=$1
  dir=/srv/releases/v$v
  n=solstice-$v-linux-$arch
  mkdir -p "$dir/$n"
  cp "$2" "$dir/$n/solstice"
  chmod 0755 "$dir/$n/solstice"
  tar -czf "$dir/$n.tar.gz" -C "$dir" "$n"
  rm -rf "${dir:?}/$n"
  if [ "${3:-}" = wrong ]; then
    printf '%064d  %s\n' 0 "$n.tar.gz" >"$dir/SHA256SUMS"
  else
    (cd "$dir" && sha256sum "$n.tar.gz" >SHA256SUMS)
  fi
  openssl pkeyutl -sign -inkey /root/key.pem -rawin -in "$dir/SHA256SUMS" -out "$dir/SHA256SUMS.sig"
}

status_file=/var/lib/solstice/update/status

# ask VERSION writes the request as the web process does, then waits for the
# helper to finish.
ask() {
  rm -f "$status_file"
  runuser -u solstice -- sh -c 'cd /var/lib/solstice/update &&
    printf "{\"version\":\"%s\",\"requestedBy\":\"test\"}\n" "$1" >.request.tmp &&
    mv .request.tmp request' sh "$1"
  i=0
  while [ "$i" -lt 300 ]; do
    if [ -f "$status_file" ] && [ "$(status)" != running ] && [ ! -e /var/lib/solstice/update/request ] &&
      [ "$(systemctl show -p ActiveState --value solstice-update.service)" != activating ]; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  echo "the helper did not finish the update to $1"
  return 1
}

status() { sed -n 's/.*"status":"\([a-z_]*\)".*/\1/p' "$status_file"; }
status_is() { [ "$(status)" = "$1" ]; }
log_has() { grep -qF "$1" "$status_file"; }
one_backup() { [ "$(find /var/lib/solstice/backups -name '*.sql.gz' -size +0 | wc -l)" -eq 1 ]; }
installed_is() { [ "$(/usr/local/bin/solstice version)" = "$1" ]; }
previous_is() { [ "$(/usr/local/lib/solstice/solstice.previous version)" = "$1" ]; }
serving() {
  systemctl is-active --quiet solstice &&
    runuser -u solstice -- /usr/local/bin/solstice health -url http://127.0.0.1:8080/healthz >/dev/null 2>&1
}

inside() {
  arch=$1
  failures=0

  echo "› releases and a throwaway signing key"
  openssl genpkey -algorithm ed25519 -out /root/key.pem 2>/dev/null
  openssl pkey -in /root/key.pem -pubout -out /root/key.pub 2>/dev/null
  mkdir -p /root/install
  cp /src/deploy/install.sh /src/deploy/solstice.service /src/deploy/solstice-update.path \
    /src/deploy/solstice-update.service /root/install/
  cp /src/bin/solstice-1.0.0 /root/install/solstice
  # The release_pubkey block of update.sh, replaced with the throwaway key.
  awk -v keyfile=/root/key.pub '
    /^release_pubkey=\047/ {
      printf "release_pubkey=\047"
      n = 0
      while ((getline line < keyfile) > 0) { if (n++) printf "\n"; printf "%s", line }
      print "\047"
      skip = 1
      next
    }
    skip && /^-----END PUBLIC KEY-----\047$/ { skip = 0; next }
    !skip' /src/deploy/update.sh >/root/install/update.sh
  if [ "$(grep -c SOLSTICE_RELEASE_KEY_PLACEHOLDER /root/install/update.sh)" -ne 1 ]; then
    echo "the release key block of update.sh was not replaced"
    exit 1
  fi
  release 1.0.1 /src/bin/solstice-1.0.1
  printf '#!/bin/sh\necho 1.0.2\n' >/root/fake-1.0.2
  release 1.0.2 /root/fake-1.0.2 wrong
  # Reports its version, as the helper checks, but never serves.
  printf '#!/bin/sh\nif [ "${1:-}" = version ]; then echo 1.0.3; exit 0; fi\necho "broken build" >&2\nexit 1\n' >/root/fake-1.0.3
  release 1.0.3 /root/fake-1.0.3

  echo "› install.sh --with-mariadb, Solstice 1.0.0"
  sh /root/install/install.sh --with-mariadb >/root/install.log 2>&1 || {
    cat /root/install.log
    exit 1
  }
  check "1.0.0 installed" installed_is 1.0.0
  check "1.0.0 serving" serving
  check "the helper marker is in place" test -f /var/lib/solstice/update/helper
  check "the update path unit is active" systemctl is-active --quiet solstice-update.path

  printf 'http://127.0.0.1:%s\n' "$port" >/usr/local/lib/solstice/update-test
  chmod 0644 /usr/local/lib/solstice/update-test
  systemd-run --quiet --unit=solstice-releases busybox httpd -f -p "127.0.0.1:$port" -h /srv/releases

  echo "› update to 1.0.1"
  ask 1.0.1
  check "status ok" status_is ok
  check "the log says the signature was verified" log_has "Signature verified."
  check "the log says the database was backed up" log_has "Backing up the database solstice"
  check "one backup written" one_backup
  check "1.0.1 installed" installed_is 1.0.1
  check "1.0.0 kept as the previous binary" previous_is 1.0.0
  check "1.0.1 serving" serving

  echo "› update to 1.0.2 (wrong checksum)"
  ask 1.0.2
  check "status failed" status_is failed
  check "the log names the checksum" log_has "Checksum mismatch for solstice-1.0.2-linux-$arch.tar.gz: refused."
  check "1.0.1 still installed" installed_is 1.0.1
  check "the previous binary is untouched" previous_is 1.0.0
  check "1.0.1 still serving" serving

  echo "› update to 1.0.3 (does not start)"
  ask 1.0.3
  check "status rolled_back" status_is rolled_back
  check "the log says it went back" log_has "is not healthy after"
  check "the log says 1.0.1 runs again" log_has "Solstice 1.0.1 is running again."
  check "1.0.1 installed again" installed_is 1.0.1
  check "1.0.1 serving again" serving

  if [ "$failures" -ne 0 ]; then
    echo "--- journal"
    journalctl -u solstice-update -u solstice --no-pager -n 120 -o cat || true
    printf '%s check(s) failed\n' "$failures"
    exit 1
  fi
  echo "all checks passed"
}

if [ "${1:-}" = --inside ]; then
  inside "$2"
else
  host
fi
