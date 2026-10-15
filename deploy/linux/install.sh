#!/bin/sh
# Installs (or upgrades) Solstice on Linux as a systemd service.
#
#   sudo ./install.sh [--with-mariadb] [--port 8080] [--binary ./solstice]
#
#   --with-mariadb   also install MariaDB from the distribution's packages and
#                    create the database and its account (random password);
#                    the setup wizard then skips its database step.
#   --port N         port Solstice listens on (default 8080).
#   --binary PATH    the solstice binary to install (default: next to this
#                    script).
#
# Running it again with a newer binary upgrades: configuration and data are
# kept, the service restarts and migrates the database by itself.
#
# With update.sh and the solstice-update units next to it (release archives
# have them), it also installs the update helper: administrators can then
# update Solstice from its web interface (Administration, « Mises à jour »).
#
# Files: /usr/local/bin/solstice, /etc/solstice/solstice.conf,
# /var/lib/solstice (attachments, keys, update requests, backups),
# /usr/local/lib/solstice (update helper, previous binary),
# /etc/systemd/system/solstice.service, solstice-update.path and .service.
# Uninstall: systemctl disable --now solstice solstice-update.path, then remove
# those files and the "solstice" system user (and the database if you created
# it).
set -eu

here=$(cd "$(dirname "$0")" && pwd)
port=8080
binary="$here/solstice"
with_db=0

say() { printf '%s\n' "$*"; }
fail() { printf 'Erreur / Error: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case $1 in
    --with-mariadb) with_db=1 ;;
    --port) port=${2:?}; shift ;;
    --binary) binary=${2:?}; shift ;;
    -h | --help) sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) fail "option inconnue / unknown option: $1" ;;
  esac
  shift
done

[ "$(id -u)" -eq 0 ] || fail "lancez ce script en root (sudo) / run as root (sudo)."
command -v systemctl >/dev/null 2>&1 || fail "systemd est requis / systemd is required."
[ -x "$binary" ] || fail "binaire introuvable / binary not found: $binary"
case $port in *[!0-9]* | '') fail "port invalide / invalid port: $port" ;; esac

secret() { od -An -N24 -tx1 /dev/urandom | tr -d ' \n'; }

# The system account and directories.
if ! id solstice >/dev/null 2>&1; then
  nologin=$(command -v nologin || echo /usr/sbin/nologin)
  useradd --system --home-dir /var/lib/solstice --no-create-home --shell "$nologin" solstice
fi
install -d -o solstice -g solstice -m 0750 /var/lib/solstice /etc/solstice

# The binary (an upgrade replaces it; the service restarts below).
upgrade=0
[ -f /usr/local/bin/solstice ] && upgrade=1
install -m 0755 "$binary" /usr/local/bin/solstice.new
mv -f /usr/local/bin/solstice.new /usr/local/bin/solstice

config=/etc/solstice/solstice.conf
if [ ! -f "$config" ]; then
  umask 077
  {
    say "# Solstice configuration (KEY=value). The setup wizard completes it."
    say "SOLSTICE_ADDR=:$port"
  } > "$config"
  chown solstice:solstice "$config"
  chmod 0600 "$config"
fi

if [ $with_db -eq 1 ] && ! grep -q '^SOLSTICE_DB=' "$config"; then
  if ! command -v mariadb >/dev/null 2>&1 && ! command -v mysql >/dev/null 2>&1; then
    say "Installation de MariaDB… / Installing MariaDB…"
    if command -v apt-get >/dev/null 2>&1; then
      DEBIAN_FRONTEND=noninteractive apt-get install -y mariadb-server
    elif command -v dnf >/dev/null 2>&1; then
      dnf install -y mariadb-server
    elif command -v zypper >/dev/null 2>&1; then
      zypper --non-interactive install mariadb
    else
      fail "installez MariaDB vous-même puis relancez sans --with-mariadb / install MariaDB yourself."
    fi
  fi
  systemctl enable --now mariadb 2>/dev/null || systemctl enable --now mysql
  client=$(command -v mariadb || command -v mysql)
  pw=$(secret)
  "$client" <<SQL
CREATE DATABASE IF NOT EXISTS solstice CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'solstice'@'localhost' IDENTIFIED BY '$pw';
CREATE USER IF NOT EXISTS 'solstice'@'127.0.0.1' IDENTIFIED BY '$pw';
ALTER USER 'solstice'@'localhost' IDENTIFIED BY '$pw';
ALTER USER 'solstice'@'127.0.0.1' IDENTIFIED BY '$pw';
GRANT ALL PRIVILEGES ON solstice.* TO 'solstice'@'localhost';
GRANT ALL PRIVILEGES ON solstice.* TO 'solstice'@'127.0.0.1';
FLUSH PRIVILEGES;
SQL
  say "SOLSTICE_DB=mysql://solstice:$pw@127.0.0.1:3306/solstice" >> "$config"
fi

install -m 0644 "$here/solstice.service" /etc/systemd/system/solstice.service

# The update helper. Solstice, unprivileged, may only drop a request in
# /var/lib/solstice/update: root's directory, writable by the solstice group,
# sticky so that Solstice cannot touch root's files there. A root service
# started by the request installs the release (see update.sh).
helper=0
if [ -f "$here/update.sh" ] && [ -f "$here/solstice-update.path" ] && [ -f "$here/solstice-update.service" ]; then
  helper=1
  install -d -o root -g root -m 0755 /usr/local/lib/solstice
  install -m 0755 "$here/update.sh" /usr/local/lib/solstice/update.sh.new
  mv -f /usr/local/lib/solstice/update.sh.new /usr/local/lib/solstice/update.sh
  install -m 0644 "$here/solstice-update.path" /etc/systemd/system/solstice-update.path
  install -m 0644 "$here/solstice-update.service" /etc/systemd/system/solstice-update.service
  for d in /var/lib/solstice/update /var/lib/solstice/backups; do
    if [ -L "$d" ] || { [ -e "$d" ] && [ ! -d "$d" ]; }; then rm -f "$d"; fi
  done
  install -d -o root -g solstice -m 1770 /var/lib/solstice/update
  install -d -o root -g root -m 0700 /var/lib/solstice/backups
  chown root:solstice /var/lib/solstice/update
  chmod 1770 /var/lib/solstice/update
  chown root:root /var/lib/solstice/backups
  chmod 0700 /var/lib/solstice/backups
fi
systemctl daemon-reload
if [ $helper -eq 1 ]; then
  systemctl enable solstice-update.path
  systemctl restart solstice-update.path
  # The marker tells Solstice that it can update itself.
  marker=$(mktemp /var/lib/solstice/update/.helper.XXXXXX)
  say "solstice-update 1" > "$marker"
  chmod 0644 "$marker"
  mv -f "$marker" /var/lib/solstice/update/helper
fi
# A stale code from a previous start must not be shown: Solstice writes a new one.
rm -f /var/lib/solstice/setup-code
if [ $upgrade -eq 1 ]; then
  systemctl restart solstice
else
  systemctl enable --now solstice
fi

# The setup code appears when Solstice starts without any account.
code=""
i=0
while [ $i -lt 60 ]; do
  if [ -s /var/lib/solstice/setup-code ]; then
    code=$(tr -d ' \n' < /var/lib/solstice/setup-code)
    break
  fi
  if /usr/local/bin/solstice health -url "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then
    [ -s /var/lib/solstice/setup-code ] && code=$(tr -d ' \n' < /var/lib/solstice/setup-code)
    break
  fi
  i=$((i + 1))
  sleep 1
done

host=$(hostname -f 2>/dev/null || hostname)
say ""
if [ -n "$code" ]; then
  say "Solstice est prêt à être installé / Solstice is ready to be set up:"
  say "  http://$host:$port/setup"
  say "  Code d'installation / Setup code: $code"
else
  say "Solstice tourne / Solstice is running: http://$host:$port/"
fi
say ""
say "Journal / Logs:   journalctl -u solstice -f"
say "Configuration:    $config"
