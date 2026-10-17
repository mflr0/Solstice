# Upgrades

Solstice migrates its database by itself when it starts, so an upgrade is
a new binary (or image) and a restart. Related pages:
[installation](install.md), [reverse proxy](reverse-proxy.md),
[backups](backups.md).

Back up before you upgrade, as described in [backups](backups.md). The
integrated update dumps the database for you; the other methods do not.

To see the running version, use `solstice version`, the updates page of the
administration, or `GET /healthz`, which answers `{"status":"ok","version":...}`.

## Integrated update (Linux, systemd)

On an installation made with the release archive's `install.sh`, global
administrators can update from the administration, on the Updates page.

Solstice checks GitHub for a new release at most every 12 hours. This is the
only request it makes on its own, and an administrator can turn the checks
off. The page shows the installed version, the latest release and its notes.
**Update to v<version>** asks the update helper to install it. The page offers
this button only when the helper is installed. In a Docker container or on an
installation without the helper, it shows the commands to run instead.

### What the helper does

Solstice itself runs as the unprivileged `solstice` user. All it does is write
a request file naming a version. The helper runs as root and does the work, in
this order:

1. Checks that the version is a valid version number and newer than the
   running one. It never installs an older release.
2. Downloads `SHA256SUMS`, `SHA256SUMS.sig` and the archive for this machine
   (`amd64` or `arm64`) from the GitHub release, over HTTPS.
3. Checks the signature of `SHA256SUMS`. It is an Ed25519 signature by the
   release key written in `/usr/local/lib/solstice/update.sh`. No checksum is
   read before the signature is valid. A release with a missing or wrong
   signature is refused.
4. Checks the checksum of the archive against `SHA256SUMS`.
5. Extracts the binary and checks that it reports the requested version.
6. Backs the database up (see below).
7. Keeps the running binary as `/usr/local/lib/solstice/solstice.previous`.
8. Replaces `/usr/local/bin/solstice` and restarts the `solstice` service.
9. Waits up to 60 seconds for the service to run and answer its health check.
10. If it does not, puts the previous binary back and restarts it. The status
    is then `rolled_back`.

A failure in any step up to 8 leaves the installed version unchanged.

The helper needs `curl` or `wget`, `openssl` 1.1.1 or later (for Ed25519),
and `setpriv` or `runuser`. It refuses every update while the release key in
`update.sh` is still the placeholder. The key is replaced when you run
`install.sh` from a newer archive, not by the updates themselves.

### The backup

Before it installs anything, the helper dumps the database to
`/var/lib/solstice/backups/<UTC time>.sql.gz` and keeps the last 5. The
directory belongs to root, mode 0700. The dump runs as the `solstice` user with
`mariadb-dump` or `mysqldump`, with `--single-transaction --quick --hex-blob
--no-tablespaces`, over TCP.

- If neither tool is installed, the helper says so in its log and updates
  without a backup.
- If no database is configured, there is nothing to back up.
- If the dump fails, or `SOLSTICE_DB_URL` cannot be read, the helper refuses
  to update.

The backup covers the database only, not the data directory or the
configuration.

### Rollback

If the new version is not healthy after 60 seconds, the helper restores the
previous binary. It does not undo changes to the database, because MariaDB
does not roll back schema changes. The helper's log then names the backup
taken before the update. To go back fully, restore it as described in
[backups](backups.md).

### Status and logs

The helper writes its progress to `/var/lib/solstice/update/status`, which the
Updates page reads. The status is `running`, `ok`, `failed` or `rolled_back`.
The same lines go to the journal:

```sh
journalctl -u solstice-update
```

### The update helper units

`install.sh` installs two systemd units beside `solstice.service`, and
`update.sh` in `/usr/local/lib/solstice`:

- `solstice-update.path` watches `/var/lib/solstice/update/request`. When that
  file appears, it starts `solstice-update.service`. `install.sh` enables it.
- `solstice-update.service` is a one-shot service that runs
  `/usr/local/lib/solstice/update.sh` as root, with up to 30 minutes to finish.
  It can write only to `/usr/local/bin`, `/usr/local/lib/solstice` and
  `/var/lib/solstice`.

The directory `/var/lib/solstice/update` belongs to root, group `solstice`, mode
1770: Solstice can drop a request there but cannot touch root's files. The file
`/var/lib/solstice/update/helper`, written by `install.sh`, tells Solstice that
the helper is installed.

Check the units:

```sh
systemctl status solstice-update.path
```

## Manual upgrade (Linux)

Use this when the helper is not installed, when it refuses an update, or when
you prefer to control the steps.

```sh
curl -fLO https://github.com/mflr0/Solstice/releases/download/v<version>/solstice-<version>-linux-amd64.tar.gz
tar -xzf solstice-<version>-linux-amd64.tar.gz
cd solstice-<version>-linux-amd64
sudo ./install.sh
```

Use `arm64` in the name on an ARM machine. Running `install.sh` again keeps the
configuration and the data, replaces the binary, the unit and the update
helper, restarts the service and migrates the database. It does not back
anything up: do that first.

Without systemd, replace the `solstice` binary and restart `solstice serve` in
whatever way you run it. It migrates the database at start; `solstice migrate`
does only that.

## Docker upgrade

The Docker image is built from the sources. To move to a release, check out
its tag and run the installer again, from the repository:

```sh
git fetch --tags
git checkout v<version>
SOLSTICE_VERSION=<version> ./deploy/docker/install.sh
```

The installer keeps `.env` and both volumes, rebuilds the image
(`docker compose up -d --build`) and restarts the containers. Solstice migrates
the database when it starts. `SOLSTICE_VERSION` is the tag of the image and the
version the build reports.

The installer prints a setup code only on an instance without accounts. For an
instance that is already set up, it prints the address.

## The `tls` option and the backup

`SOLSTICE_DB_URL` accepts one option, `?tls=` (see [installation](install.md)),
to encrypt Solstice's own link to the database. The update helper reads the
database address from the configuration file but ignores everything after the
`?`. It passes no TLS option to the dump tool. Two effects follow:

- Whether the backup link is encrypted depends on the dump tool's own defaults
  and not on `?tls=`.
- If the database server refuses connections the dump tool makes, the dump
  fails and the helper refuses to update, with the message "The database backup
  failed: refusing to update without one."

In that case, take a dump yourself with your tool's TLS options (see
[backups](backups.md)), then upgrade with the manual method above.

The same applies to a database address that the helper cannot read: it refuses
to update without a backup. The helper reads `mysql://` and `mariadb://`
addresses only.
