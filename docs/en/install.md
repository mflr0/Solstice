# Installation

This page covers installing Solstice on a Linux server with systemd, or with
Docker, then the setup wizard and the configuration. Related pages:
[reverse proxy](reverse-proxy.md), [backups](backups.md),
[upgrades](upgrades.md).

Solstice is one binary. It stores its data in MariaDB 10.6+ or MySQL 8.0+ and
in a data directory on disk (attachments, the key of the stored secrets, the
setup code). It listens in plain HTTP: put a reverse proxy in front of it for
HTTPS.

## Linux (systemd)

Download a release archive, extract it and run the installer as root:

```sh
tar xzf solstice-<version>-linux-amd64.tar.gz
cd solstice-<version>-linux-amd64
sudo ./install.sh --with-mariadb
```

The installer needs systemd and takes these options:

| Option | Effect |
|---|---|
| `--with-mariadb` | Installs MariaDB from the distribution's packages if no `mariadb` or `mysql` client is found (`apt-get`, `dnf` or `zypper`), creates the `solstice` database and account with a random password, and writes `SOLSTICE_DB_URL` to the configuration file. The setup wizard then skips its database step. |
| `--port N` | Port Solstice listens on. Default `8080`. Written as `SOLSTICE_HTTP_ADDR=:N` when the installer creates the configuration file. |
| `--binary PATH` | The `solstice` binary to install. Default: the one next to the script. |

Without `--with-mariadb`, create the database yourself and give its address to
the setup wizard. The installer uses this statement, which you can adapt:

```sql
CREATE DATABASE solstice CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### What the installer does

- Creates the `solstice` system user.
- Installs the binary as `/usr/local/bin/solstice`.
- Creates `/etc/solstice/solstice.conf` (mode 0600, owned by `solstice`) if it
  does not exist.
- Creates `/var/lib/solstice`, the data directory.
- Installs the systemd unit `/etc/systemd/system/solstice.service`, then
  enables and starts it.
- Installs the update helper when `update.sh`, `solstice-update.path` and
  `solstice-update.service` are next to the script, as they are in release
  archives: `/usr/local/lib/solstice/update.sh` and the two units in
  `/etc/systemd/system`. See [upgrades](upgrades.md).
- Waits for Solstice to answer, then prints the address of the setup wizard
  and the setup code.

Running the installer again with a newer archive upgrades in place: the
configuration and the data are kept, the service restarts and the database is
migrated at start.

### The service

`solstice.service` runs `/usr/local/bin/solstice serve` as the `solstice`
user. It sets `SOLSTICE_CONFIG=/etc/solstice/solstice.conf` and
`SOLSTICE_DATA=/var/lib/solstice` in its environment. Environment variables
win over the file, so a `SOLSTICE_DATA` line in the file has no effect under
this unit.

The unit restarts Solstice after a failure (`Restart=on-failure`, after 5
seconds) and allows 30 seconds to stop (`TimeoutStopSec=30`). It starts after
`network-online.target`, `mariadb.service` and `mysql.service`. It is
hardened: no new privileges, no capabilities, the system read-only except
`/var/lib/solstice` and `/etc/solstice`, and home directories read-only.

```sh
systemctl status solstice
journalctl -u solstice -f
```

### Uninstall

```sh
systemctl disable --now solstice solstice-update.path
```

Then remove the files listed above, the `solstice` system user and, if you
created it, the database.

## Docker

The Compose file in `deploy/docker/compose.yaml` builds the image from the
sources and starts it with a MariaDB 11.4 container.

```sh
git clone https://github.com/mflr0/Solstice.git
cd Solstice/deploy/docker
./install.sh
```

`install.sh` needs Docker and Docker Compose v2. It does this:

1. Writes `.env` next to the Compose file if it does not exist, with random
   `DB_PASSWORD` and `DB_ROOT_PASSWORD` (hexadecimal, mode 0600) and
   `SOLSTICE_PORT` (default `8080`). Keep this file.
2. Runs `docker compose up -d --build`.
3. Waits for Solstice, then prints the address of the setup wizard and the
   setup code.

To do it by hand, create `.env` with `DB_PASSWORD` and `DB_ROOT_PASSWORD`
(letters and digits only), run `docker compose up -d`, and read the setup code
with `docker compose logs solstice`.

What the Compose file sets up:

- Two services, `mariadb` and `solstice`, and two volumes: `db` (the
  database) and `data` (mounted at `/var/lib/solstice`, the data directory).
- `SOLSTICE_DB_URL` points to the `mariadb` service, so the setup wizard
  skips its database step.
- The image sets `SOLSTICE_DATA=/var/lib/solstice`,
  `SOLSTICE_CONFIG=/var/lib/solstice/solstice.conf`,
  `SOLSTICE_HTTP_ADDR=:8080`, `SOLSTICE_INSTALL=docker` and
  `SOLSTICE_LOG_FORMAT=json`. The configuration file therefore lives in the
  `data` volume.
- The port is published as `${SOLSTICE_PORT:-8080}:8080`. Use
  `"127.0.0.1:8080:8080"` instead if only a reverse proxy on the same machine
  must reach Solstice.
- The image has a health check that runs `solstice health` every 30 seconds.
- To import from Jira in the wizard, uncomment the `jira-home` volume line in
  `compose.yaml`, then give `/import/jira-home` as the path in the wizard.

To set another `SOLSTICE_*` setting, add it under `environment:` of the
`solstice` service in `compose.yaml`.

```sh
docker compose -f compose.yaml logs -f solstice
docker compose -f compose.yaml down
```

## The setup wizard

When no database is configured, or the database has no account yet, Solstice
serves only the setup wizard. Pages redirect to `/setup`, and `/api` answers
503, until setup is done. `/healthz` keeps answering.

The wizard is locked by a one-time code of eight characters, written like
`4F7K-9QX2`. Solstice:

- prints it in a frame in its log;
- writes it to `<data>/setup-code` (mode 0600), which is
  `/var/lib/solstice/setup-code` on Linux;
- draws a new one at every start, until setup is done;
- deletes the file when the first administrator exists.

Whoever can read the log or the data directory is the administrator, so
protect both. Wrong codes are throttled per client address.

The steps:

1. Open `/setup` and type the code.
2. Database: asked only if `SOLSTICE_DB_URL` is not set. The wizard tests the
   connection before it saves it.
3. Public address and time zone. The public address is `http://` or
   `https://`, a host and an optional port, without a path, for example
   `https://solstice.example.org`. The time zone is an IANA name, for example
   `Europe/Paris`.
4. Create the administrator, or import a Jira instance.

The wizard writes `SOLSTICE_DB_URL`, `SOLSTICE_DATA`, `SOLSTICE_HTTP_BASEURL`
and `SOLSTICE_TZ` to the configuration file. If the file cannot be written,
it shows the lines to paste into it. When the first administrator is created,
Solstice switches to the application without a restart.

## Configuration

Settings are `KEY=value` lines in a file. Solstice looks for the file in this
order and uses the first that exists:

1. the path in the `SOLSTICE_CONFIG` environment variable;
2. `./solstice.conf`;
3. `/etc/solstice/solstice.conf`.

Rules for the file:

- Blank lines and lines starting with `#` are ignored.
- `export ` before a key is allowed.
- A value wrapped in matching single or double quotes loses them.
- If a key appears twice, the later line wins.
- An empty value counts as not set.
- The same file can be read by systemd (`EnvironmentFile`) and by Docker
  (`--env-file`). The wizard therefore writes only values without quotes,
  backslashes, `$`, backticks or control characters, and without leading or
  trailing spaces.

Environment variables of the same name win over the file. Solstice reads the
file at start: restart it after a change. A `SOLSTICE_` key that Solstice does
not know, in the file or the environment, is reported in the log at start.
`solstice -h` lists the settings.

Durations are written `30s`, `5m`, `1h30m`.

| Key | Default | Meaning |
|---|---|---|
| `SOLSTICE_DB_URL` | none | The database, `mysql://user:password@host:3306/database`. See below. |
| `SOLSTICE_DB_MAXOPENCONNS` | `20` | Open connections, at most. At least 1. |
| `SOLSTICE_DB_MAXIDLECONNS` | `10` | Idle connections kept. Between 0 and `SOLSTICE_DB_MAXOPENCONNS`. |
| `SOLSTICE_DB_CONNMAXLIFETIME` | `5m` | Lifetime of a connection. Must be positive. |
| `SOLSTICE_DATA` | `./data` | Data directory: attachments, `secret.key`, `setup-code`. |
| `SOLSTICE_TZ` | the system's | Time zone of dates typed in searches, for example `Europe/Paris`. |
| `SOLSTICE_SECRETKEY` | `<data>/secret.key` | Key of the stored secrets. See [backups](backups.md). |
| `SOLSTICE_INSTALL` | none | `docker` in the image: the update page explains how to update a container. |
| `SOLSTICE_HTTP_ADDR` | `:8080` | Listen address. |
| `SOLSTICE_HTTP_BASEURL` | none | Public address, for example `https://solstice.example.org`. See [reverse proxy](reverse-proxy.md). |
| `SOLSTICE_HTTP_TRUSTEDPROXIES` | none | Reverse proxies whose `X-Forwarded-For` is believed. See [reverse proxy](reverse-proxy.md). |
| `SOLSTICE_HTTP_SHUTDOWNTIMEOUT` | `10s` | Time given to requests and e-mails in progress when Solstice stops. Must be positive. |
| `SOLSTICE_HTTP_MAXUPLOAD` | `100MiB` | Size of one upload. `KB`, `MB`, `GB` count in 1000s, `KiB`, `MiB`, `GiB` in 1024s. Raise the proxy's limit too. |
| `SOLSTICE_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `SOLSTICE_LOG_FORMAT` | `text` | `text` or `json` (`json` in the Docker image). |

`SOLSTICE_CONFIG` itself is read only from the environment. There is no
default database: without `SOLSTICE_DB_URL`, the wizard asks for one.

### The database address

`SOLSTICE_DB_URL` is `mysql://user:password@host:3306/database`. The scheme
`mariadb://` is accepted too. Percent-encode special characters of the
password: `@` becomes `%40`.

To encrypt the link to a database on another machine, add one `tls` option:

| Option | Effect |
|---|---|
| `?tls=true` | Encrypted, certificate checked. |
| `?tls=skip-verify` | Encrypted, certificate not checked. |
| `?tls=preferred` | Encrypted when the server offers it, certificate not checked. |
| `?tls=false` | Not encrypted. This is the default. |

No other option after `?` is accepted, and Solstice refuses to start with
one. Error messages never print the password. The effect of `?tls=` on the
update helper's backup is described in [upgrades](upgrades.md#the-tls-option-and-the-backup).

### Health check

`GET /healthz` answers 200 when the database responds and 503 otherwise. It
also answers during setup. `solstice health` makes this request against
`SOLSTICE_HTTP_ADDR` and exits with status 0 on a 200 answer; `-url` gives
another address.

### Other commands

```sh
solstice migrate                          # update the database schema
solstice user add -username me -admin     # create an account (password printed)
solstice user passwd -username me         # change a password
solstice version
```

`solstice serve` migrates the database at start, so `migrate` is rarely
needed. Run these commands with the same configuration as the service, for
example with `SOLSTICE_CONFIG=/etc/solstice/solstice.conf` in the
environment.
