#!/bin/sh
set -eu

# Test TLS option mapping for MariaDB and MySQL dump clients
# as specified in GitHub issue #32

dump_tls_opts() {
  dump_bin=$1
  tls_val=$2

  case $($dump_bin --version 2>/dev/null) in
    *MariaDB*)
      case $tls_val in
        true) printf '%s\n' "--ssl --ssl-verify-server-cert" ;;
        skip-verify | preferred) printf '%s\n' "--ssl --skip-ssl-verify-server-cert" ;;
        false) printf '%s\n' "--skip-ssl" ;;
        "") return 0 ;;
      esac
      ;;
    *)
      case $tls_val in
        true) printf '%s\n' "--ssl-mode=VERIFY_IDENTITY" ;;
        skip-verify) printf '%s\n' "--ssl-mode=REQUIRED" ;;
        preferred) printf '%s\n' "--ssl-mode=PREFERRED" ;;
        false) printf '%s\n' "--ssl-mode=DISABLED" ;;
        "") return 0 ;;
      esac
      ;;
  esac
}

failures=0
pass() { printf 'ok   %s\n' "$1"; }
fail() { printf 'FAIL %s\n' "$1"; failures=$((failures + 1)); }

mariadb_dump() { printf 'mariadb-dump  Ver 11.4.0-MariaDB\n'; }
mysql_dump() { printf 'mysqldump  Ver 8.0.41 for Linux on x86_64\n'; }

expect_opts() {
  name=$1
  dump=$2
  tls=$3
  expected=$4
  got=$(dump_tls_opts "$dump" "$tls")
  if [ "$got" = "$expected" ]; then pass "$name"; else fail "$name (got '$got', expected '$expected')"; fi
}

# Eight combinations plus "absent"
expect_opts "MariaDB true" mariadb_dump true "--ssl --ssl-verify-server-cert"
expect_opts "MariaDB skip-verify" mariadb_dump skip-verify "--ssl --skip-ssl-verify-server-cert"
expect_opts "MariaDB preferred" mariadb_dump preferred "--ssl --skip-ssl-verify-server-cert"
expect_opts "MariaDB false" mariadb_dump false "--skip-ssl"
expect_opts "MariaDB absent" mariadb_dump "" ""
expect_opts "MySQL true" mysql_dump true "--ssl-mode=VERIFY_IDENTITY"
expect_opts "MySQL skip-verify" mysql_dump skip-verify "--ssl-mode=REQUIRED"
expect_opts "MySQL preferred" mysql_dump preferred "--ssl-mode=PREFERRED"
expect_opts "MySQL false" mysql_dump false "--ssl-mode=DISABLED"
expect_opts "MySQL absent" mysql_dump "" ""

if [ "$failures" -ne 0 ]; then
  printf '%s check(s) failed\n' "$failures"
  exit 1
fi
echo "all checks passed"
