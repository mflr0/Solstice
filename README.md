# Solstice

[Français](README.fr.md)

Solstice is a self-hosted issue tracker. Projects carry their own permissions
and workflows, boards cover Kanban and Scrum, search works with plain filters
or JQL, and pages update by themselves when someone else changes an issue.
It can take over an existing Jira Data Center or Server instance by reading its
database directly, history included.

One Go binary serves the API and the embedded web interface (Svelte 5 and
TypeScript). Data lives in MariaDB 10.6+ or MySQL 8.0+.

## Features

- Issues: sub-tasks, links, attachments, comments with restricted visibility,
  time tracking, watchers, votes, custom fields and a full change history. The
  rich-text editor stores descriptions and comments as wiki markup, unchanged.
- Projects without shared schemes: permissions, issue security and workflows
  belong to each project. The workflow editor shows statuses, a transition
  matrix and a live diagram.
- Boards: Kanban and Scrum, columns with WIP limits, swimlanes, quick filters,
  backlog and sprints. Cards move by drag and drop or from the keyboard.
- Search: a filter bar with counts per value, search by person and by role,
  a JQL editor with completion (history operators `WAS` and `CHANGED`
  included), saved filters with e-mail subscriptions, CSV export.
- Notifications by e-mail, sent immediately (changes to one issue are grouped)
  or as a daily digest, with mentions and due-date reminders. Webhooks.
- Integrations: a REST API compatible with Jira's `/rest/api/2` and
  `/rest/agile/1.0`, so existing scripts and tools keep working; personal
  access tokens; an audit log.
- Interface: eight themes with light and dark modes, in French, English,
  German, Italian and Hindi, usable on a phone.

## Installation

### With Docker (recommended)

```bash
git clone https://github.com/mflr0/Solstice.git
cd Solstice/deploy/docker
./install.sh
```

The script writes random database passwords to `.env`, starts MariaDB and
Solstice, then prints the address of the setup wizard and its one-time code.
Running it again keeps the data and restarts on the current sources.

### On a Linux server (systemd)

From a release archive:

```bash
tar xzf solstice-<version>-linux-amd64.tar.gz
cd solstice-<version>-linux-amd64
sudo ./install.sh --with-mariadb
```

The script creates a `solstice` system user, `/etc/solstice` and
`/var/lib/solstice`, and a hardened systemd service. `--with-mariadb` also
installs MariaDB and creates the database with its account; without it, the
setup wizard asks for an existing database. Running the script again with a
newer archive upgrades in place.

### First start: the setup wizard

When no database is configured, or the database has no account yet, Solstice
only serves its setup wizard at `/setup`. It prints a one-time code in its log
(also written to `<data>/setup-code`). The wizard asks for the database unless
one is already set, then the public address and time zone, and finally
creates the administrator or imports a Jira instance.

### Configuration

Settings live in a file of `KEY=value` lines, found through `-config`, then
`SOLSTICE_CONFIG`, then `./solstice.conf`, then `/etc/solstice/solstice.conf`.
Environment variables of the same name take precedence; `solstice -h` lists
them. `GET /healthz` answers 200 when the database responds and 503 otherwise.

Put an HTTPS reverse proxy in front of Solstice. The session cookie becomes
`Secure` behind a proxy that sends `X-Forwarded-Proto: https`.

The SMTP password and webhook secrets are encrypted with the key in
`SOLSTICE_SECRET_KEY` (32 bytes in base64 or hex, for example
`openssl rand -base64 32`), or else with `<data>/secret.key`, created when
first needed. Back up this key along with the database and the data
directory: without it, those secrets have to be entered again.

## Migrating from Jira

The setup wizard can import a Jira instance, and so can the command line:

```bash
solstice import \
  -from postgres://jira:secret@db-jira:5432/jira \
  -tz Europe/Paris \
  -jira-home /var/atlassian/application-data/jira
```

- `-from`: the Jira database, `postgres://…` or `mysql://…`. Read access is
  enough. For a consistent result, stop Jira first or import from a copy of
  its database.
- `-tz`: the time zone of the Jira server (Administration › System info).
  MySQL stores dates without a time zone, and dates without a time (due
  dates, versions) depend on it with both engines.
- `-jira-home`: optional; copies the attachment files. Without it, only their
  metadata is imported.
- `-replace`: overwrites existing Solstice data (otherwise the import refuses
  a non-empty database). The import can be run again: identifiers are kept.

The final report lists what was imported and what deserves a look. Imported:

- Accounts: users (from the directory with priority), groups (nested groups
  flattened), administrators. Internal accounts keep their passwords: `{PKCS5S2}`
  hashes are converted to Argon2id at the first sign-in. Accounts from an
  external directory (LDAP…) have no password: set one with
  `solstice user passwd`. Accounts without application access are imported
  as inactive.
- Projects: categories, leads, old keys of renamed projects, roles and their
  members, available issue types, components, versions.
- Permissions: each project gets its own copy of its permission scheme and
  issue security scheme. Whatever cannot be translated is removed, never
  widened: a confidential issue never becomes visible to more people than
  before.
- Workflows: each project gets its own copy of the workflows of its scheme
  (statuses and transitions; the built-in system workflow, which is not
  stored in the database, is recreated). Conditions, validators and post
  functions are Java code and are not imported: Solstice applies its own
  rules (permissions, resolution set when an issue reaches a done status).
- Issues: fields, sub-tasks, links, remote links, labels, watchers, votes,
  comments with their visibility restrictions, work logs, attachments, custom
  fields, full history, old keys of moved issues.
- Boards and sprints, with the board order (rank) and epics; saved filters
  with their shares and favourites.

Not imported: dashboards, and notification schemes (Solstice has its own
notification settings).

## E-mail and webhooks

- SMTP server: set in the administration (host, port, security none /
  STARTTLS / TLS, user name, sender), with a test message that shows the
  server's exact error. The public address (`SOLSTICE_BASE_URL`) is used for
  links in e-mails; without it, e-mails go out without links.
- Notifications: watchers, reporter, assignee and mentioned people
  (`[~username]`). Each person picks their events, immediate delivery
  (changes to the same issue less than two minutes apart go in one e-mail) or
  a daily digest, and can ask for a morning reminder of their due issues. A
  project can turn its notifications off or add a group or role to the
  recipients. Permissions are checked when each e-mail is written: nobody
  receives an issue or a restricted comment they cannot see.
- Filter subscriptions: the results of a saved filter, every day or on some
  days of the week, as each recipient sees them (200 issues at most, with a
  link to the rest).
- Webhooks (administration): events and payloads in the format of Jira Server
  webhooks (`jira:issue_updated`, `comment_created`, `sprint_started`…), an
  optional JQL filter, a `X-Hub-Signature: sha256=…` signature (HMAC-SHA256
  of the body) when a secret is set, spaced retries, a log of the last 100
  deliveries. Targets may be internal (private network, local machine), as
  self-hosted integrations often are. Link-local ranges (169.254.0.0/16,
  fe80::/10) and cloud metadata addresses (169.254.169.254, fd00:ec2::254…)
  are always refused, checked after DNS resolution on every connection;
  redirects are not followed and no proxy is used. A webhook sees every
  issue, confidential ones included: restrict it with its JQL filter.

## Development

Requirements: Go 1.27+, Node 22+ with pnpm, Docker (or OrbStack).

```bash
docker compose up -d          # MariaDB for Solstice, plus test PostgreSQL and MySQL Jira databases
corepack enable pnpm
(cd frontend && pnpm install)
```

Then, in two terminals (Solstice has no default database: give the
development one from `compose.yaml` explicitly, or put it in
`backend/solstice.conf`):

```bash
export SOLSTICE_DB=mysql://solstice:solstice@localhost:13306/solstice
cd backend && go run ./cmd/solstice serve      # API on http://localhost:8080
cd frontend && pnpm dev                       # interface on http://localhost:5173
```

On a database without accounts the server opens the setup wizard (`/setup`,
code in its log). Otherwise create a first account:
`go run ./cmd/solstice user add -username me -name "Me" -admin` (the generated
password is printed).

`make build` produces the single binary with the interface embedded,
`make docker` the image, and `make release VERSION=x.y.z` the archives for
Linux, macOS and Windows with their SHA256SUMS.

### Tests

```bash
(cd backend && go test ./...)          # unit tests
(cd frontend && pnpm test && pnpm lint)
./scripts/e2e-import.sh                # end to end, with the containers of compose.yaml
```

`scripts/e2e-import.sh` generates the same synthetic Jira database on
PostgreSQL and on MySQL (`backend/cmd/jira-fixture`, from the
`entitymodel.xml` of a Jira distribution placed next to the repository),
imports both, checks that the results are identical table by table and match
what was generated, then runs the API tests (permissions, confidentiality,
real time…) against the imported database.

### Layout

```
backend/
  cmd/solstice/         the binary: serve, migrate, import, user, version, health
  cmd/jira-fixture/     generator of synthetic Jira databases (tests)
  internal/api/         JSON HTTP API used by the interface, REST compatibility layer
  internal/importer/    import from Jira
  internal/jiradb/      connection to Jira databases (PostgreSQL, MySQL)
  internal/perm/        who sees and does what
  internal/search/      the simple search
  internal/jql/         JQL
  internal/live/        real-time updates (SSE)
  internal/events/      event bus: real time, e-mail, webhooks
  internal/notify/      e-mail notifications, digests, subscriptions, reminders
  internal/mail/        SMTP settings, sending, e-mail queue
  internal/webhook/     webhooks, SSRF guard
  internal/secret/      encryption of stored secrets
  internal/settings/    instance settings stored in the database
  internal/config/      the configuration file
  internal/audit/       audit log
  internal/i18n/        server messages in five languages
  internal/auth/        passwords and sessions
  internal/db/          MariaDB connection and migrations
  internal/web/         serves the built interface
frontend/
  src/core/             pure, tested logic (wiki markup rendering, JQL…)
  src/ui/               Svelte components
  src/lib/              state, router, API client
  src/i18n/             interface texts, one file per language
  src/themes/           the themes and their contract (ANATOMY.md)
deploy/                 Docker and Linux installation
scripts/                end-to-end import test, release archives
```

## Design choices

- No schemes. Permissions, issue security and issue types are set in the
  project, on one page. Changing one project never touches another.
- Fewer, broader permissions. Resolving and closing are part of "change the
  status"; everyone can edit or delete their own comments.
- Administrators configure, they don't read everything: being an
  administrator gives no access to projects or confidential issues.
- Sub-tasks always follow the security level of their parent.
- Sign-in: after five failures for the same user name or the same address,
  attempts are suspended for a minute, then longer.

## Roadmap

- 1.0, in progress: an administration area to set everything from the
  interface (projects, components, versions, fields, groups, instance
  settings), and updates applied from the administration.
- 1.1: dashboards and agile reports, bulk editing and keyboard shortcuts,
  LDAP/Active Directory and single sign-on.

## Contributing and AI tools

AI tools are welcome in contributions, under one rule: whoever submits a
change owns it. Every pull request must be understood by its author, who has
tested it and can explain any part of it. If a contribution turns out to be a
problem (a bug, malicious code, a license violation), its author is held
solely responsible, never the tool. Solstice itself is developed with the
help of AI tools, under the same rule.

The full rules, and what happens when they are broken, are in the
[code of conduct](CODE_OF_CONDUCT.md).

## License

Solstice is free software under the GNU Affero General Public License,
version 3 (AGPL-3.0): see [LICENSE](LICENSE). You may use, study, modify and
share it. If you run a modified version for users over a network, you must
offer them its source code.

## Trademarks

Jira and Atlassian are trademarks of Atlassian. Solstice is an independent
project, not affiliated with or endorsed by Atlassian. To read Jira databases,
the importer relies on the schema description (`entitymodel.xml`) of a Jira
distribution placed next to the repository during development; it is not
versioned, and no Atlassian code, style or visual is used in Solstice.
