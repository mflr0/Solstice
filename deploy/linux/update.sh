#!/bin/sh
# Solstice update helper: installs a released version of Solstice when an
# administrator asks for it in the web interface.
#
# solstice-update.path starts solstice-update.service (root, oneshot), which
# runs this script, as soon as /var/lib/solstice/update/request exists.
# The web process writes that file; this script removes it, reports what it
# does in /var/lib/solstice/update/status (JSON), and logs to the journal.
#
# Steps: validate the requested version; download the archive for this
# machine, SHA256SUMS and its signature SHA256SUMS.sig from the release on
# GitHub; verify the signature of SHA256SUMS against the public key written
# in this script, then the checksum of the archive; extract; check that the binary reports the requested version; back the
# database up (mariadb-dump or mysqldump, if installed), with the encryption
# the tls option of SOLSTICE_DB_URL asks for, to
# /var/lib/solstice/backups, keeping the last 5; keep the running binary as
# /usr/local/lib/solstice/solstice.previous; swap the binary atomically;
# restart Solstice and wait for it to be healthy; otherwise put the previous
# binary back and restart it (status "rolled_back").
#
# Threat model. The web process runs as the unprivileged user "solstice" and
# may be compromised. It can write the request, the configuration file
# (/etc/solstice/solstice.conf) and anything under /var/lib/solstice. This
# script runs as root, so it must not let that user gain anything more:
#  - The request is data, never code. The only value taken from it is the
#    version, matched against a strict semantic-version pattern. It is never
#    executed, sourced or evaluated, and never echoed back.
#  - Downloads come only from the GitHub releases of mflr0/Solstice, over
#    HTTPS. Redirects are followed one by one, and only to GitHub's download
#    hosts. SHA256SUMS must carry a valid Ed25519 signature (SHA256SUMS.sig)
#    by the release key, whose public half is written in this script
#    (release_pubkey): before the signature is checked no checksum is read.
#    The archive must then match its line in SHA256SUMS, and the binary
#    inside must report the requested version. The checksum catches corrupt
#    or truncated downloads; the signature, made with a key that never
#    leaves the maintainer, is what a compromised GitHub account cannot
#    forge. A missing or bad signature, an openssl without Ed25519, or the
#    placeholder key (a copy of this script that has no key yet) refuses the
#    update.
#  - Only a newer release than the running one is installed: no downgrade to
#    an older, vulnerable release.
#  - Directories the web process can write to are entered once, checked by
#    owner and mode, then used through relative paths only. Renaming or
#    replacing them afterwards cannot redirect root's writes. Files written
#    in them are created with mktemp (exclusive, never through a symbolic
#    link) and then renamed into place. The work directory is private to
#    this script (PrivateTmp in the unit).
#  - The configuration file is parsed, never sourced. The database URL and
#    the listen address are validated field by field. The downloaded binary
#    and the database dump run as the "solstice" user, not as root. The
#    database password reaches the dump tool in its environment, never on a
#    command line. The dump connects with the encryption the tls option of
#    the URL asks for; a value the server would not take stops the update.
#  - The test override of the download address (a loopback HTTP server) is
#    read only from a root-owned file in a root-owned directory, out of the
#    web process's reach: /usr/local/lib/solstice/update-test.
# A compromised web process can still prevent updates or ask for the latest
# release again and again. It can disrupt the service anyway.
set -eu
umask 077
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
LC_ALL=C
export PATH LC_ALL

base=https://github.com/mflr0/Solstice/releases/download
allowed="https://github.com/ https://objects.githubusercontent.com/ https://release-assets.githubusercontent.com/"
protocols='=https'
update_dir=/var/lib/solstice/update
backup_dir=/var/lib/solstice/backups
lib=/usr/local/lib/solstice
bin=/usr/local/bin/solstice
conf=/etc/solstice/solstice.conf
test_file=$lib/update-test
service=solstice
account=solstice
keep_backups=5
health_wait=60
semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?$'
nl='
'

work=""
log=""
started=""
from=unknown
to=unknown
state=""
backup=""
db_tls=""

have() { command -v "$1" >/dev/null 2>&1; }

# begin signature
# The release signing key: the Ed25519 public key (PEM) whose private half
# signs SHA256SUMS in scripts/release.sh. PLACEHOLDER: the maintainer replaces
# the whole block with the output of
#   openssl pkey -in <private key> -pubout
# (see the top of scripts/release.sh). While the marker below is present,
# this helper refuses every update.
release_pubkey='-----BEGIN PUBLIC KEY-----
SOLSTICE_RELEASE_KEY_PLACEHOLDER
-----END PUBLIC KEY-----'
placeholder_mark=SOLSTICE_RELEASE_KEY_PLACEHOLDER

# release_key_set fails, saying why, while release_pubkey is the placeholder.
release_key_set() {
  case $release_pubkey in
    *"$placeholder_mark"*)
      say "This update helper has no release signing key yet (placeholder): updates are refused until the maintainer's public key is set in $0."
      return 1
      ;;
  esac
}

# verify_signature $1 $2 succeeds only if file $2 is a valid Ed25519
# signature of file $1 by release_pubkey. It says why on failure. The work
# directory ($work) holds the key file.
verify_signature() {
  release_key_set || return 1
  if [ -L "$2" ] || [ ! -f "$2" ] || [ ! -s "$2" ]; then
    say "The release has no signature (SHA256SUMS.sig): refused."
    return 1
  fi
  if ! have openssl; then
    say "openssl is required to verify the release signature."
    return 1
  fi
  # The key file is private to this run; Ed25519 signatures are 64 bytes.
  keyfile=$work/release-key.pub
  printf '%s\n' "$release_pubkey" >"$keyfile" || return 1
  if ! openssl pkey -pubin -in "$keyfile" -noout -text_pub 2>/dev/null | grep -q 'ED25519' ||
    ! openssl pkeyutl -help 2>&1 | grep -q -- '-rawin'; then
    say "This openssl cannot verify Ed25519 signatures (OpenSSL 1.1.1 or later is required): update refused."
    return 1
  fi
  if [ "$(wc -c <"$2")" -ne 64 ]; then
    say "SHA256SUMS.sig is not an Ed25519 signature: refused."
    return 1
  fi
  if ! openssl pkeyutl -verify -pubin -inkey "$keyfile" -rawin -in "$1" -sigfile "$2" >/dev/null 2>&1; then
    say "The signature of SHA256SUMS is wrong: refused."
    return 1
  fi
  return 0
}
# end signature

# say writes a line to the journal and to the log shown in the interface.
say() {
  printf '%s\n' "$*" >&2
  if [ -n "$log" ]; then
    printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*" >>"$log"
  fi
}

# json_escape turns its input into the inside of a JSON string: printable
# ASCII only, lines joined with \n.
json_escape() {
  tr '\011' ' ' | tr -cd '\012\040-\176' | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' |
    awk 'BEGIN { ORS = "" } NR > 1 { print "\\n" } { print }'
}

# write_status records the state in the status file the interface reads.
# The current directory is the checked update directory.
write_status() {
  state=$1
  tmp=$(mktemp ./.status.XXXXXX) || return 1
  {
    printf '{"at":"%s","from":"%s","to":"%s","status":"%s","log":"' "$started" "$from" "$to" "$state"
    if [ -n "$log" ] && [ -f "$log" ]; then tail -n 300 "$log" | json_escape; fi
    printf '"}\n'
  } >"$tmp" && chmod 0644 "$tmp" || {
    rm -f "$tmp"
    return 1
  }
  # Anything but a regular file in the way (planted by the web process).
  if [ -L status ] || { [ -e status ] && [ ! -f status ]; }; then rm -rf -- status; fi
  mv -f -- "$tmp" status
}

# fail ends an update that changed nothing.
fail() {
  say "$*"
  say "Update to $to failed; Solstice $from is unchanged."
  write_status failed || true
  exit 0
}

finish() {
  # Stopped half-way (a bug, a signal): say so rather than "running" forever.
  if [ "$state" = running ]; then
    say "The update helper stopped unexpectedly."
    write_status failed || true
  fi
  if [ -n "$work" ]; then rm -rf -- "$work"; fi
}

# check_dir tells whether the current directory has this owner and mode.
check_dir() { [ "$(stat -c '%u %a' .)" = "$1" ]; }

# as_user runs a command as the solstice user.
as_user() {
  if have setpriv; then
    setpriv --reuid="$account" --regid="$account" --init-groups -- "$@"
  elif have runuser; then
    runuser -u "$account" -- "$@"
  else
    say "setpriv or runuser is required."
    return 1
  fi
}

valid_version() {
  [ "${#1}" -le 64 ] && printf '%s\n' "$1" | grep -Eq "$semver"
}

# newer tells whether version $1 is newer than $2 (semantic versioning).
newer() {
  awk -v a="$1" -v b="$2" '
    function cmpid(x, y,   xn, yn) {
      xn = x ~ /^[0-9]+$/; yn = y ~ /^[0-9]+$/
      if (xn && yn) {
        if (length(x) != length(y)) return length(x) < length(y) ? -1 : 1
        return x "" < y "" ? -1 : (x "" > y "" ? 1 : 0)
      }
      if (xn) return -1
      if (yn) return 1
      return x "" < y "" ? -1 : (x "" > y "" ? 1 : 0)
    }
    function cmpv(a, b,   ac, bc, ap, bp, i, n, m, c, x, y) {
      ac = a; ap = ""; i = index(a, "-"); if (i) { ac = substr(a, 1, i - 1); ap = substr(a, i + 1) }
      bc = b; bp = ""; i = index(b, "-"); if (i) { bc = substr(b, 1, i - 1); bp = substr(b, i + 1) }
      split(ac, x, "."); split(bc, y, ".")
      for (i = 1; i <= 3; i++) { c = cmpid(x[i], y[i]); if (c) return c }
      if (ap == "" && bp == "") return 0
      if (ap == "") return 1
      if (bp == "") return -1
      n = split(ap, x, "."); m = split(bp, y, ".")
      for (i = 1; i <= n && i <= m; i++) { c = cmpid(x[i], y[i]); if (c) return c }
      return n < m ? -1 : (n > m ? 1 : 0)
    }
    BEGIN { exit !(cmpv(a, b) > 0) }'
}

# read_request prints the requested version, or fails. Nothing else is
# taken from the file, and its content is never shown.
read_request() {
  if [ -L request ] || [ ! -f request ]; then return 1; fi
  # At most 4 KiB, and no waiting on a pipe planted in its place.
  if have timeout; then
    content=$(timeout 10 dd if=request bs=1024 count=4 2>/dev/null) || return 1
  else
    content=$(dd if=request bs=1024 count=4 2>/dev/null) || return 1
  fi
  case $content in *"$nl"*) return 1 ;; esac
  v=$(printf '%s\n' "$content" | sed -n 's/^{.*"version":"\([^"]*\)".*}$/\1/p')
  valid_version "$v" || return 1
  printf '%s\n' "$v"
}

# conf_value prints a setting of the configuration file, parsed as Solstice
# does (KEY=value, # comments, optional "export " and quotes), never sourced.
conf_value() {
  if [ -L "$conf" ] || [ ! -f "$conf" ]; then return 0; fi
  awk -v k="$1" '
    { line = $0; sub(/^[ \t]+/, "", line); sub(/[ \t\r]+$/, "", line) }
    line == "" || line ~ /^#/ { next }
    {
      i = index(line, "="); if (!i) next
      key = substr(line, 1, i - 1); val = substr(line, i + 1)
      sub(/^export /, "", key); gsub(/^[ \t]+|[ \t]+$/, "", key)
      if (key != k) next
      gsub(/^[ \t]+|[ \t]+$/, "", val)
      q = substr(val, 1, 1)
      if (length(val) >= 2 && (q == "\"" || q == "\047") && substr(val, length(val), 1) == q) val = substr(val, 2, length(val) - 2)
      found = val; has = 1
    }
    END { if (has) print found }' "$conf"
}

# read_test_base prints the download address of test mode, if set up.
read_test_base() {
  if [ -L "$test_file" ] || [ ! -f "$test_file" ]; then return 1; fi
  if [ "$(stat -c '%u' "$lib")" != 0 ] || [ "$(stat -c '%u' "$test_file")" != 0 ] ||
    [ -n "$(find "$lib" "$test_file" -maxdepth 0 \( -perm -020 -o -perm -002 \))" ]; then
    say "Ignoring $test_file: it must belong to root and be writable by root only."
    return 1
  fi
  u=$(sed -n '1p' "$test_file")
  if ! printf '%s\n' "$u" | grep -Eq '^http://(127\.0\.0\.1|localhost):[0-9]{1,5}(/[A-Za-z0-9._~-]+)*$'; then
    say "Ignoring $test_file: not a loopback address."
    return 1
  fi
  printf '%s\n' "$u"
}

allowed_url() {
  case $1 in *[!A-Za-z0-9._~:/?\&=%+-]*) return 1 ;; esac
  for prefix in $allowed; do
    case $1 in "$prefix"*) return 0 ;; esac
  done
  return 1
}

# fetch downloads a URL to a file of at most $3 bytes, following redirects
# itself so that each address is checked.
fetch() {
  url=$1 out=$2 max=$3 hops=0
  while :; do
    if ! allowed_url "$url"; then
      say "Refused to download from an unexpected address."
      return 1
    fi
    rm -f "$out"
    if have curl; then
      res=$(curl -sS --proto "$protocols" --max-redirs 0 --connect-timeout 20 --max-time 900 \
        --max-filesize "$max" -A Solstice-update -o "$out" -w '%{http_code} %{redirect_url}' "$url" 2>>"$log") || {
        say "Download failed: $(basename "$out")."
        return 1
      }
      code=${res%% *}
      next=${res#* }
    elif have wget; then
      # GNU wget: -S prints the answer's headers, read for the redirect.
      wget -q -S --max-redirect=0 --timeout=60 --tries=2 -U Solstice-update -O "$out" "$url" 2>"$work/headers" || true
      code=$(sed -n 's/^ *HTTP\/[0-9.]* \([0-9][0-9][0-9]\).*/\1/p' "$work/headers" | tail -n 1)
      next=$(sed -n 's/^ *[Ll]ocation: *//p' "$work/headers" | tail -n 1 | tr -d '\r')
      if [ -z "$code" ]; then
        cat "$work/headers" >>"$log"
        say "Download failed: $(basename "$out")."
        return 1
      fi
    else
      say "curl or wget is required."
      return 1
    fi
    if [ -f "$out" ] && [ "$(wc -c <"$out")" -gt "$max" ]; then
      say "Download too large: $(basename "$out")."
      return 1
    fi
    case $code in
      200) return 0 ;;
      301 | 302 | 303 | 307 | 308) ;;
      *)
        say "Download failed: $(basename "$out") (HTTP $code)."
        return 1
        ;;
    esac
    hops=$((hops + 1))
    if [ "$hops" -gt 5 ]; then
      say "Too many redirects."
      return 1
    fi
    url=$next
  done
}

# read_db sets db_user, db_pass, db_host, db_port and db_name from
# SOLSTICE_DB_URL. Returns 2 when no database is configured.
read_db() {
  url=$(conf_value SOLSTICE_DB_URL)
  [ -n "$url" ] || return 2
  parsed=$(printf '%s\n' "$url" | awk '
    function hex(c) { return index("0123456789abcdef", tolower(c)) - 1 }
    function decode(s,   out, i, c, h, l) {
      out = ""
      for (i = 1; i <= length(s); i++) {
        c = substr(s, i, 1)
        if (c == "%") {
          h = hex(substr(s, i + 1, 1)); l = hex(substr(s, i + 2, 1))
          if (h < 0 || l < 0) { bad = 1; return "" }
          c = sprintf("%c", h * 16 + l); i += 2
          if (c == "\n" || c == "") { bad = 1; return "" }
        }
        out = out c
      }
      return out
    }
    NR > 1 { exit 1 }
    {
      u = $0
      if (u !~ /^(mysql|mariadb):\/\//) exit 1
      sub(/^[a-z]+:\/\//, "", u)
      q = index(u, "?"); if (q) u = substr(u, 1, q - 1)
      at = 0
      for (i = length(u); i > 0; i--) if (substr(u, i, 1) == "@") { at = i; break }
      if (!at) exit 1
      cred = substr(u, 1, at - 1); rest = substr(u, at + 1)
      c = index(cred, ":")
      if (c) { user = substr(cred, 1, c - 1); pass = substr(cred, c + 1) } else { user = cred; pass = "" }
      s = index(rest, "/"); if (!s) exit 1
      hostport = substr(rest, 1, s - 1); name = substr(rest, s + 1)
      if (hostport ~ /^\[/) {
        e = index(hostport, "]"); if (!e) exit 1
        host = substr(hostport, 2, e - 2); port = substr(hostport, e + 1); sub(/^:/, "", port)
      } else {
        c = index(hostport, ":")
        if (c) { host = substr(hostport, 1, c - 1); port = substr(hostport, c + 1) } else { host = hostport; port = "" }
      }
      if (port == "") port = "3306"
      user = decode(user); pass = decode(pass)
      if (bad) exit 1
      print user; print pass; print host; print port; print name
    }') || return 1
  db_user=$(printf '%s\n' "$parsed" | sed -n 1p)
  db_pass=$(printf '%s\n' "$parsed" | sed -n 2p)
  db_host=$(printf '%s\n' "$parsed" | sed -n 3p)
  db_port=$(printf '%s\n' "$parsed" | sed -n 4p)
  db_name=$(printf '%s\n' "$parsed" | sed -n 5p)
  [ -n "$db_user" ] &&
    printf '%s\n' "$db_host" | grep -Eq '^[A-Za-z0-9._:][A-Za-z0-9._:-]{0,252}$' &&
    printf '%s\n' "$db_port" | grep -Eq '^[0-9]{1,5}$' && [ "$db_port" -ge 1 ] && [ "$db_port" -le 65535 ] &&
    printf '%s\n' "$db_name" | grep -Eq '^[A-Za-z0-9_][A-Za-z0-9_-]{0,63}$'
}

# read_db_tls prints the tls option of SOLSTICE_DB_URL, or nothing when the
# URL has none. It fails when the value is not one the server accepts: the
# update must stop rather than back the database up over a link the server
# would never have used.
read_db_tls() {
  url=$(conf_value SOLSTICE_DB_URL)
  query=${url#*\?}
  [ "$query" != "$url" ] || return 0
  found=0
  tls=
  remaining=$query
  while [ -n "$remaining" ]; do
    case $remaining in
      *\&*) param=${remaining%%&*}; remaining=${remaining#*&} ;;
      *) param=$remaining; remaining="" ;;
    esac
    case $param in
      tls | tls=*) found=1; tls=${param#tls}; tls=${tls#=} ;;
    esac
  done
  if [ "$found" = 0 ]; then return 0; fi
  # The server lower-cases the value (backend/internal/db/params.go).
  tls=$(printf '%s\n' "$tls" | tr '[:upper:]' '[:lower:]')
  case $tls in
    true | false | skip-verify | preferred) printf '%s\n' "$tls" ;;
    *) return 1 ;;
  esac
}

# dump_tls_opts prints the dump options for the tls value of SOLSTICE_DB_URL.
# MariaDB clients (their --version says MariaDB) check the server certificate
# by default since 11.4, so skip-verify and preferred turn the check off;
# MySQL clients take --ssl-mode. For preferred on MariaDB there is no --ssl:
# that keeps the client's own default (TLS when the server offers it, plain
# otherwise), as the driver's preferred does; --ssl would make the backup
# fail on a server without TLS.
dump_tls_opts() {
  dump_bin=$1
  tls_val=$2

  case $($dump_bin --version 2>/dev/null) in
    *MariaDB*)
      case $tls_val in
        true) printf '%s\n' "--ssl --ssl-verify-server-cert" ;;
        skip-verify) printf '%s\n' "--ssl --skip-ssl-verify-server-cert" ;;
        preferred) printf '%s\n' "--skip-ssl-verify-server-cert" ;;
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

# back_up dumps the database to $backup_dir/<time>.sql.gz and keeps the last
# few. Runs in a subshell: it changes directory and holds the password.
back_up() (
  dump=$(command -v mariadb-dump || command -v mysqldump || true)
  if [ -z "$dump" ]; then
    say "Neither mariadb-dump nor mysqldump is installed: the database is not backed up."
    exit 0
  fi
  status=0
  read_db || status=$?
  case $status in
    0) ;;
    2)
      say "No database configured: nothing to back up."
      exit 0
      ;;
    *)
      say "SOLSTICE_DB_URL in $conf cannot be read: refusing to update without a backup."
      exit 1
      ;;
  esac
  mkdir -p -m 0700 "$backup_dir" && cd -P "$backup_dir" || exit 1
  case $(pwd) in */solstice/backups) ;; *) exit 1 ;; esac
  if ! check_dir "0 700"; then
    say "$backup_dir must belong to root with mode 0700: refusing to write the backup there."
    exit 1
  fi
  stamp=$(date -u +%Y%m%dT%H%M%SZ)
  tmp=$(mktemp ./.dump.XXXXXX) || exit 1
  say "Backing up the database $db_name to $backup_dir/$stamp.sql.gz."
  MYSQL_PWD=$db_pass
  export MYSQL_PWD
  set -- --single-transaction --quick --hex-blob --no-tablespaces --protocol=TCP
  # The options contain no globbing characters; split into words on purpose.
  # shellcheck disable=SC2046
  set -- "$@" $(dump_tls_opts "$dump" "$db_tls")
  set -- "$@" --host="$db_host" --port="$db_port" --user="$db_user" "$db_name"
  if ! as_user "$dump" "$@" >"$tmp" 2>>"$log"; then
    rm -f "$tmp"
    say "The database backup failed: refusing to update without one."
    exit 1
  fi
  unset MYSQL_PWD
  if ! gzip "$tmp" || ! mv -f -- "$tmp.gz" "$stamp.sql.gz"; then
    rm -f "$tmp" "$tmp.gz"
    exit 1
  fi
  # shellcheck disable=SC2010 # names are checked by the pattern
  ls -1 | grep -E '^[0-9]{8}T[0-9]{6}Z\.sql\.gz$' | sort -r | awk -v keep="$keep_backups" 'NR > keep' |
    while IFS= read -r old; do rm -f -- "$old"; done
  printf '%s\n' "$backup_dir/$stamp.sql.gz" >"$work/backup"
)

# healthy waits for the service to run and answer its health check.
healthy() {
  i=0
  while [ "$i" -lt "$health_wait" ]; do
    sleep 2
    i=$((i + 2))
    if systemctl is-active --quiet "$service" &&
      as_user env SOLSTICE_HTTP_ADDR="$addr" "$bin" health >/dev/null 2>&1; then
      return 0
    fi
  done
  return 1
}

# install_binary puts a binary in place by renaming, so the swap is atomic.
install_binary() {
  cp -- "$1" /usr/local/bin/.solstice.update &&
    chmod 0755 /usr/local/bin/.solstice.update &&
    mv -f -- /usr/local/bin/.solstice.update "$bin"
}

main() {
  # The request directory: root's, group-writable for the web process,
  # sticky so that it cannot touch root's files.
  if ! cd -P "$update_dir" 2>/dev/null; then
    say "$update_dir does not exist."
    exit 0
  fi
  case $(pwd) in */solstice/update) ;; *) exit 1 ;; esac
  if ! check_dir "0 1770"; then
    say "$update_dir must belong to root with mode 1770: refusing to act."
    exit 1
  fi
  if [ ! -e request ] && [ ! -L request ]; then exit 0; fi

  started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  work=$(mktemp -d /var/tmp/solstice-update.XXXXXX)
  trap finish EXIT
  trap 'exit 1' HUP INT TERM
  log=$work/log
  : >"$log"

  if ! to=$(read_request); then
    to=unknown
    rm -f request
    fail "Invalid update request: ignored."
  fi
  from=$(as_user "$bin" version 2>/dev/null | head -n 1) || from=""
  case $from in "" | *[!0-9A-Za-z.+-]*) from=unknown ;; esac
  from=$(printf '%.64s' "$from")
  say "Update requested: Solstice $from to $to."
  # The request goes even if the status cannot be written: one left behind
  # would start this service again and again.
  write_status running || true
  rm -f request

  if ! valid_version "$from"; then
    fail "The installed version ($from) is not a release: update it by hand."
  fi
  if ! newer "$to" "$from"; then
    fail "$to is not newer than $from: refused."
  fi
  case $(uname -m) in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) fail "No release is built for this machine ($(uname -m))." ;;
  esac
  addr=$(conf_value SOLSTICE_HTTP_ADDR)
  addr=${addr:-:8080}
  if ! printf '%s\n' "$addr" | grep -Eq '^[][A-Za-z0-9.:-]{0,253}:[0-9]{1,5}$'; then
    fail "SOLSTICE_HTTP_ADDR in $conf is not an address."
  fi
  # A value the server would not take stops the update before any download.
  if ! db_tls=$(read_db_tls); then
    fail "The tls option of SOLSTICE_DB_URL in $conf is not true, false, skip-verify or preferred."
  fi
  if test_base=$(read_test_base); then
    base=$test_base
    allowed=$(printf '%s\n' "$test_base" | sed 's#^\(http://[^/]*\).*#\1/#')
    protocols='=http,https'
    say "Test mode: downloading from $base."
  fi

  release_key_set || fail "Updates cannot be verified."
  name=solstice-$to-linux-$arch
  say "Downloading $name.tar.gz."
  fetch "$base/v$to/SHA256SUMS" "$work/SHA256SUMS" 65536 || fail "Could not download SHA256SUMS."
  fetch "$base/v$to/SHA256SUMS.sig" "$work/SHA256SUMS.sig" 1024 || fail "Could not download SHA256SUMS.sig: a release without a signature is refused."
  # Nothing in SHA256SUMS is trusted before this.
  verify_signature "$work/SHA256SUMS" "$work/SHA256SUMS.sig" || fail "The signature of the release could not be verified."
  say "Signature verified."
  fetch "$base/v$to/$name.tar.gz" "$work/$name.tar.gz" 268435456 || fail "Could not download $name.tar.gz."
  want=$(awk -v n="$name.tar.gz" '($2 == n || $2 == "*" n) && length($1) == 64 && $1 ~ /^[0-9a-f]+$/ { print $1 }' "$work/SHA256SUMS")
  if [ -z "$want" ] || [ "$(printf '%s\n' "$want" | wc -l)" -ne 1 ]; then
    fail "SHA256SUMS has no checksum for $name.tar.gz."
  fi
  got=$(sha256sum "$work/$name.tar.gz" | cut -d ' ' -f 1)
  if [ "$got" != "$want" ]; then
    fail "Checksum mismatch for $name.tar.gz: refused."
  fi
  say "Checksum verified."

  mkdir "$work/x"
  if ! tar -xzf "$work/$name.tar.gz" -C "$work/x" --no-same-owner "$name/solstice" 2>>"$log"; then
    fail "The archive cannot be extracted."
  fi
  new=$work/x/$name/solstice
  if [ -L "$new" ] || [ ! -f "$new" ]; then fail "The archive holds no solstice binary."; fi
  # The unprivileged user runs it: it must reach it, not change it.
  chmod 0755 "$work" "$work/x" "$work/x/$name" "$new"
  reported=$(as_user "$new" version 2>>"$log" | head -n 1) || reported=""
  if [ "$reported" != "$to" ]; then
    fail "The downloaded binary does not report version $to."
  fi

  back_up || fail "No backup, no update."
  if [ -f "$work/backup" ]; then backup=$(cat "$work/backup"); fi

  if ! cp -p -- "$bin" "$lib/.solstice.previous" || ! mv -f -- "$lib/.solstice.previous" "$lib/solstice.previous"; then
    fail "Could not keep the current binary."
  fi
  if ! install_binary "$new"; then
    install_binary "$lib/solstice.previous" || true
    fail "Could not install the new binary."
  fi
  say "Restarting Solstice $to."
  systemctl restart "$service" >>"$log" 2>&1 || say "systemctl restart $service failed."
  if healthy; then
    say "Solstice $to is running and healthy."
    write_status ok
    exit 0
  fi

  say "Solstice $to is not healthy after ${health_wait} s: going back to $from."
  journalctl -u "$service" -n 20 --no-pager -o cat >>"$log" 2>&1 || true
  if [ -n "$backup" ]; then
    say "If $to changed the database, the backup taken before is $backup."
  fi
  if install_binary "$lib/solstice.previous" && systemctl restart "$service" >>"$log" 2>&1 && healthy; then
    say "Solstice $from is running again."
    write_status rolled_back
  else
    say "Solstice $from does not start either: see journalctl -u $service."
    write_status failed
  fi
  exit 0
}

main "$@"
