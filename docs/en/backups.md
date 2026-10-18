# Backups

Solstice keeps its state in three places. Back up all three, and restore them
together. Related pages: [installation](install.md),
[reverse proxy](reverse-proxy.md), [upgrades](upgrades.md).

## What to back up

| What | Where | Holds |
|---|---|---|
| The database | MariaDB or MySQL, named by `SOLSTICE_DB_URL` | Projects, issues, comments, users, settings, the audit log. |
| The data directory | `SOLSTICE_DATA`: `/var/lib/solstice` on Linux and in the Docker image | `attachments/` (the files attached to issues) and `secret.key`. |
| The configuration | `/etc/solstice/solstice.conf` on Linux; `solstice.conf` in the data directory with Docker | Settings, including the database password. |

Notes:

- Attachment files are on disk in `<data>/attachments`, not in the database.
- With Docker, the configuration file is in the `data` volume, so the data
  directory covers it. Also keep `deploy/docker/.env`, which holds the
  database passwords, and `compose.yaml` if you edited it.
- On Linux, the data directory also holds the helper's `update/` and
  `backups/` directories. `backups/` is readable by root only and contains the
  database dumps taken before updates (see [upgrades](upgrades.md)). They are
  not a backup plan: the helper takes one only when you update, and keeps the
  last 5.
- `<data>/setup-code` exists only until setup is done.
- If you set `SOLSTICE_SECRETKEY` in the environment rather than in the
  configuration file, keep a copy of that key too.
- Take the dump and the copy of the data directory at about the same time, so
  that the attachments in the files match the database.
- Keep `secret.key` apart from the database dumps. See
  [what secret.key protects](#what-secretkey-protects).

## Dump the database

The update helper dumps the database with these options, and so can you.
`mariadb-dump` is the MariaDB tool; with MySQL use `mysqldump`, which takes the
same options.

```sh
mariadb-dump --single-transaction --quick --hex-blob --no-tablespaces \
  --host=127.0.0.1 --port=3306 --user=solstice --password solstice \
  | gzip > solstice-$(date -u +%Y%m%dT%H%M%SZ).sql.gz
```

The host, port, user and database name are in `SOLSTICE_DB_URL`. `--password`
asks for the password, which is in the same URL (percent-encoded there: `%40`
is `@`). If your database requires an encrypted link, add your dump tool's own
TLS options: Solstice's `?tls=` option does not apply to it.

With Docker, run the dump inside the `mariadb` container. It already has the
database name, user and password in its environment. Run it from
`deploy/docker`:

```sh
docker compose exec -T mariadb sh -c \
  'MYSQL_PWD="$MARIADB_PASSWORD" mariadb-dump --single-transaction --quick --hex-blob --no-tablespaces --user="$MARIADB_USER" "$MARIADB_DATABASE"' \
  | gzip > solstice.sql.gz
```

## Copy the data directory and the configuration

On Linux, as root, to keep owners and modes:

```sh
tar czf solstice-data.tar.gz -C /var/lib/solstice .
cp -p /etc/solstice/solstice.conf solstice.conf.backup
```

The archive and the copy hold secrets: store them with the same care as the
database.

With Docker, copy the data directory out of the `solstice` container. This
includes the configuration file:

```sh
docker compose cp solstice:/var/lib/solstice ./solstice-data
```

## Restore

Restore into the same Solstice version that took the dump, or a newer one.
Solstice applies the migrations that are missing when it starts.

On Linux:

1. Stop Solstice:

   ```sh
   systemctl stop solstice
   ```

2. Start from an empty database. To drop and create it again, with the
   statement the installer uses (this deletes what the database holds):

   ```sql
   DROP DATABASE solstice;
   CREATE DATABASE solstice CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
   ```

   The account that `SOLSTICE_DB_URL` names must be able to use it, as it was
   before.

3. Load the dump:

   ```sh
   gunzip -c solstice-20260101T000000Z.sql.gz \
     | mariadb --host=127.0.0.1 --port=3306 --user=solstice --password solstice
   ```

4. Restore the data directory and the configuration. Extracted by root,
   the archive keeps the owners it was made with (`solstice`):

   ```sh
   tar xzf solstice-data.tar.gz -C /var/lib/solstice
   install -o solstice -g solstice -m 0600 solstice.conf.backup /etc/solstice/solstice.conf
   ```

5. Start Solstice and check it:

   ```sh
   systemctl start solstice
   curl http://127.0.0.1:8080/healthz
   ```

With Docker, from `deploy/docker`:

1. Stop Solstice, keeping MariaDB running:

   ```sh
   docker compose stop solstice
   ```

2. Empty the database and load the dump:

   ```sh
   docker compose exec -T mariadb sh -c \
     'MYSQL_PWD="$MARIADB_ROOT_PASSWORD" mariadb --user=root -e "DROP DATABASE solstice; CREATE DATABASE solstice CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"'
   gunzip -c solstice.sql.gz | docker compose exec -T mariadb sh -c \
     'MYSQL_PWD="$MARIADB_PASSWORD" mariadb --user="$MARIADB_USER" "$MARIADB_DATABASE"'
   ```

3. Put the saved files back into the `data` volume. Docker keeps the volume
   `solstice_data`; `docker volume inspect solstice_data` shows where. The
   files must belong to user and group 65532, the user the image runs as.

4. Start Solstice:

   ```sh
   docker compose start solstice
   ```

To restore on a new server, install Solstice first (see
[installation](install.md)) and do not run the setup wizard. Restore as above,
then start Solstice. The restored database has accounts, so Solstice opens the
application and not the wizard. If the new server's database address differs
from the old one, keep the `SOLSTICE_DB_URL` of the new configuration file.

## What secret.key protects

Solstice encrypts the secrets it stores in the database: the SMTP password and
the webhook secrets. It uses AES-256-GCM with a 32-byte key, so that a copy of
the database alone does not reveal them.

The key comes from:

1. `SOLSTICE_SECRETKEY`, when it is set: 32 bytes in base64 or hexadecimal, for
   example from `openssl rand -base64 32`;
2. otherwise `<data>/secret.key`. Solstice creates it the first time it needs
   it, with mode 0600. It holds the key in base64, so you can copy it into
   `SOLSTICE_SECRETKEY`.

If you lose the key, those secrets cannot be read again. Nothing else is
lost: enter the SMTP password and the webhook secrets again in the
administration. If `secret.key` is missing, Solstice creates a new one, and
the secrets sealed with the old key stay unreadable.

Because the key and the sealed secrets are meant to be separate, do not store
`secret.key` next to the database dump in the same place. A restored database
without its key works, but the SMTP and webhook secrets must be entered again.
