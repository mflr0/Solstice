#!/bin/sh
# Tests the release signature check of update.sh and the refusals of
# scripts/release.sh, with a throwaway key pair made in a temporary directory.
# Nothing here touches the system, and no key is ever kept.
#
#   sh deploy/linux/test-update-signature.sh
#
# Needs openssl 1.1.1 or later. Exits 0 when every check passes.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
update="$here/update.sh"
release="$here/../../scripts/release.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
failures=0

pass() { printf 'ok   %s\n' "$1"; }
fail() {
  printf 'FAIL %s\n' "$1"
  failures=$((failures + 1))
}
# expect_ok / expect_refused run a command quietly and check its status.
expect_ok() {
  name=$1
  shift
  if "$@" >"$tmp/out" 2>&1; then pass "$name"; else
    fail "$name"
    cat "$tmp/out"
  fi
}
expect_refused() {
  name=$1
  shift
  if "$@" >"$tmp/out" 2>&1; then fail "$name (accepted)"; else pass "$name"; fi
}

# The signature code of update.sh, taken from the script itself.
sed -n '/^# begin signature$/,/^# end signature$/p' "$update" >"$tmp/signature.sh"
[ -s "$tmp/signature.sh" ] || {
  echo "no signature block found in update.sh"
  exit 1
}
have() { command -v "$1" >/dev/null 2>&1; }
say() { printf '%s\n' "$*" >&2; }
work=$tmp
# shellcheck disable=SC1091
. "$tmp/signature.sh"

printf 'abc123  solstice-1.0.0-linux-amd64.tar.gz\n' >"$tmp/SHA256SUMS"

# 1. The copy of the script in the repository still has the placeholder key:
#    it must refuse every update.
expect_refused "placeholder key refuses" verify_signature "$tmp/SHA256SUMS" "$tmp/none"
expect_refused "placeholder key refuses the check made before downloading" release_key_set
if verify_signature "$tmp/SHA256SUMS" "$tmp/none" 2>&1 | grep -q placeholder; then pass "placeholder refusal says why"; else fail "placeholder refusal says why"; fi

# 2. Throwaway key pairs.
openssl genpkey -algorithm ed25519 -out "$tmp/good.pem" 2>/dev/null
openssl pkey -in "$tmp/good.pem" -pubout -out "$tmp/good.pub" 2>/dev/null
openssl genpkey -algorithm ed25519 -out "$tmp/other.pem" 2>/dev/null
openssl genpkey -algorithm rsa -pkeyopt rsa_keygen_bits:2048 -out "$tmp/rsa.pem" 2>/dev/null
openssl pkey -in "$tmp/rsa.pem" -pubout -out "$tmp/rsa.pub" 2>/dev/null

# The signing commands of scripts/release.sh.
sign() { openssl pkeyutl -sign -inkey "$1" -rawin -in "$2" -out "$3"; }
sign "$tmp/good.pem" "$tmp/SHA256SUMS" "$tmp/SHA256SUMS.sig"
sign "$tmp/other.pem" "$tmp/SHA256SUMS" "$tmp/other.sig"
release_pubkey=$(cat "$tmp/good.pub")

expect_ok "good signature accepted" verify_signature "$tmp/SHA256SUMS" "$tmp/SHA256SUMS.sig"

cp "$tmp/SHA256SUMS" "$tmp/tampered"
printf 'ffff00  solstice-1.0.0-linux-amd64.tar.gz\n' >"$tmp/tampered"
expect_refused "tampered SHA256SUMS refused" verify_signature "$tmp/tampered" "$tmp/SHA256SUMS.sig"

expect_refused "missing signature refused" verify_signature "$tmp/SHA256SUMS" "$tmp/absent.sig"
: >"$tmp/empty.sig"
expect_refused "empty signature refused" verify_signature "$tmp/SHA256SUMS" "$tmp/empty.sig"
head -c 63 "$tmp/SHA256SUMS.sig" >"$tmp/short.sig"
expect_refused "truncated signature refused" verify_signature "$tmp/SHA256SUMS" "$tmp/short.sig"
expect_refused "signature by another key refused" verify_signature "$tmp/SHA256SUMS" "$tmp/other.sig"
ln -sf "$tmp/SHA256SUMS.sig" "$tmp/link.sig"
expect_refused "signature that is a symbolic link refused" verify_signature "$tmp/SHA256SUMS" "$tmp/link.sig"

release_pubkey=$(cat "$tmp/rsa.pub")
expect_refused "embedded key that is not Ed25519 refused" verify_signature "$tmp/SHA256SUMS" "$tmp/SHA256SUMS.sig"
release_pubkey='not a key'
expect_refused "unreadable embedded key refused" verify_signature "$tmp/SHA256SUMS" "$tmp/SHA256SUMS.sig"

# 3. The tls option of SOLSTICE_DB_URL: update.sh reads it for the database
#    backup and refuses a value the server would not accept.
sed -n '/^conf_value() {/,/^}/p' "$update" >"$tmp/db.sh"
sed -n '/^read_db_tls() {/,/^}/p' "$update" >>"$tmp/db.sh"
if [ ! -s "$tmp/db.sh" ]; then
  echo "no database URL functions found in update.sh"
  exit 1
fi
# shellcheck disable=SC1091
conf=$tmp/solstice.conf
. "$tmp/db.sh"

tls_got=
tls_status=0
read_tls() {
  printf 'SOLSTICE_DB_URL=%s\n' "$1" >"$conf"
  tls_status=0
  tls_got=$(read_db_tls) || tls_status=$?
}
# expect_tls NAME URL EXPECTED: EXPECTED is the value read_db_tls prints, or
# the word refused when it must fail.
expect_tls() {
  read_tls "$2"
  case $3 in
    refused)
      if [ "$tls_status" -ne 0 ]; then pass "$1"; else fail "$1 (accepted)"; fi
      ;;
    *)
      if [ "$tls_status" -eq 0 ] && [ "$tls_got" = "$3" ]; then pass "$1"; else
        fail "$1 (status $tls_status, value '$tls_got')"
      fi
      ;;
  esac
}

expect_tls "a URL without tls reads as empty" "mysql://solstice:secret@db.example.org:3306/solstice" ""
expect_tls "tls=false is read" "mysql://solstice:secret@db.example.org:3306/solstice?tls=false" "false"
expect_tls "tls=true is read" "mariadb://solstice:secret@db/solstice?tls=true" "true"
expect_tls "tls=skip-verify is read" "mysql://solstice:secret@db/solstice?tls=skip-verify" "skip-verify"
expect_tls "tls=preferred is read" "mysql://solstice:secret@db/solstice?tls=preferred" "preferred"
expect_tls "tls among other options is read" \
  "mysql://solstice:secret@db/solstice?charset=utf8mb4&tls=true" "true"
expect_tls "an unknown tls value is refused" "mysql://solstice:secret@db/solstice?tls=sometimes" refused
expect_tls "an empty tls value is refused" "mysql://solstice:secret@db/solstice?tls=" refused

# 4. The dump options for each tls value, in the dialect of the dump client.
sed -n '/^dump_tls_opts() {/,/^}/p' "$update" >"$tmp/dump.sh"
[ -s "$tmp/dump.sh" ] || {
  echo "no dump_tls_opts found in update.sh"
  exit 1
}
# shellcheck disable=SC1091
. "$tmp/dump.sh"
mariadb_dump() { printf 'mariadb-dump from 11.4.5-MariaDB, client 10.19\n'; }
mysql_dump() { printf 'mysqldump  Ver 8.0.41 for Linux on x86_64\n'; }
# expect_opts NAME DUMP TLS EXPECTED
expect_opts() {
  opts_got=$(dump_tls_opts "$2" "$3")
  if [ "$opts_got" = "$4" ]; then pass "$1"; else fail "$1 (got '$opts_got')"; fi
}
expect_opts "MariaDB, tls=true checks the certificate" mariadb_dump true "--ssl --ssl-verify-server-cert"
expect_opts "MariaDB, tls=skip-verify" mariadb_dump skip-verify "--ssl --skip-ssl-verify-server-cert"
expect_opts "MariaDB, tls=preferred" mariadb_dump preferred "--ssl --skip-ssl-verify-server-cert"
expect_opts "MariaDB, tls=false" mariadb_dump false "--skip-ssl"
expect_opts "MariaDB, no tls" mariadb_dump "" ""
expect_opts "MySQL, tls=true checks the certificate" mysql_dump true "--ssl-mode=VERIFY_IDENTITY"
expect_opts "MySQL, tls=skip-verify" mysql_dump skip-verify "--ssl-mode=REQUIRED"
expect_opts "MySQL, tls=preferred" mysql_dump preferred "--ssl-mode=PREFERRED"
expect_opts "MySQL, tls=false" mysql_dump false "--ssl-mode=DISABLED"
expect_opts "MySQL, no tls" mysql_dump "" ""

# 5. scripts/release.sh refuses to run without a usable private key, before
#    it builds or deletes anything.
expect_refused "release.sh without SOLSTICE_RELEASE_KEY refuses" env -u SOLSTICE_RELEASE_KEY sh "$release" 0.0.0
expect_refused "release.sh with a missing key file refuses" env SOLSTICE_RELEASE_KEY="$tmp/absent.pem" sh "$release" 0.0.0
expect_refused "release.sh with an RSA key refuses" env SOLSTICE_RELEASE_KEY="$tmp/rsa.pem" sh "$release" 0.0.0

if [ "$failures" -ne 0 ]; then
  printf '%s check(s) failed\n' "$failures"
  exit 1
fi
echo "all checks passed"
