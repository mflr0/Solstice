# Solstice: what remains

Status on 2026-10-15, branch `feat/v1.0`.

## In progress (interrupted by a usage limit)

These three pieces of work stopped mid-way; their files are committed as they
were. Current state: the backend builds and passes `go vet`; the frontend unit
tests pass (651), but `svelte-check` reports 190 type errors, almost all
missing catalogue strings (`hooksAdmin`, `catalogAdmin`, some `account` keys):
the admin pages exist, their texts do not yet.

### 1. Administration API (backend)

- Contract: `backend/internal/api/ADMIN_API.md`; migration `0010_admin.sql`.
- Covered by the contract: general and public settings, profile (`PATCH /api/me`),
  users, groups, project roles, projects (create, update, archive, delete,
  categories, components, versions with release and merge), catalogues (issue
  types, priorities, resolutions, link types), custom fields, system
  information, import from Jira from the administration, and what a replacing
  import keeps.
- Stopped while wiring `backend/cmd/solstice/main.go`.
- To do: check each endpoint against the contract, finish the wiring, tests
  (permissions included), full `./scripts/e2e-import.sh` run.

### 2. Administration and personal settings (frontend)

- Admin area `src/ui/admin/`: overview, settings, banner, users, groups, roles,
  projects, catalogues, statuses, custom fields, mail, webhooks, tokens, audit,
  import, system, updates.
- Personal settings `src/ui/account/`: profile, appearance, notifications
  (must stay at `/settings/notifications`: e-mails link there), subscriptions,
  tokens.
- To do:
  - add the missing strings to the five catalogues (`fr`, `en`, `de`, `it`,
    `hi`): webhooks (`hooksAdmin`), catalogues (`catalogAdmin`), account pages;
  - project settings tabs: general, components, versions, notifications;
  - show the instance name and the announcement banner in the app shell;
  - check the skins (Carnet, Orbite, Rétro at least) in light and dark, at
    1440 and 390 px;
  - `pnpm lint` and `pnpm test` green.

### 3. Integrated updates

- Done so far: update check and apply API, `deploy/linux/update.sh` with
  `solstice-update.path` and `solstice-update.service`, installer and release
  script ship them, the Docker image sets `SOLSTICE_INSTALL=docker`, admin page
  `src/ui/admin/UpdatesPage.svelte`.
- To do: its strings (9 type errors), Go tests, a run of the helper in a
  systemd container (update succeeds, bad checksum refused, unhealthy version
  rolled back), link from the admin navigation, "update available" badge in
  the account menu for administrators.

## Remaining for 1.0

- Security review and hardening: every endpoint (including the new admin and
  update ones), XSS in wiki rendering and in the editor, uploads, sessions and
  CSRF, security headers (CSP, framing, referrer), `govulncheck` and
  `pnpm audit`.
- Documentation in English and French: installation (Docker, Linux, from
  source), administration (configuration, backups and restore, upgrades,
  import, e-mail, webhooks, REST API, languages, themes), `SECURITY.md`.
- Fix the known bugs below.
- Clean-up: leftover test databases (`solstice_restagent`, `jira_restagent`,
  and `solstice_admb` / `solstice_admui` if they remain), the old `solstice`
  demo database and its test issues, `.cache/` scratch directories, stray Vite
  dev servers.
- Final verification: fresh install from a release archive and from Docker
  through the setup wizard, a Jira import, backup and restore, end-to-end
  suite, lint, tests.
- Publish: privacy audit of the whole history, merge `feat/v1.0` into `main`,
  tag `v1.0.0`, GitHub release with the archives and `SHA256SUMS` (the updater
  needs them), then a one-line installer (`curl … | sudo sh`) fetching the
  latest release.

## Known bugs and polish

- Search results table: the summary column collapses to about 22 px around
  1024 px wide.
- The "More" filter popover (320 px) opens partly off-screen at 1280 px.
- `/projects/KEY/settings` scrolls horizontally at 390 px.
- Top bar navigation is clipped at 390 px.
- Theme gallery previews: long labels overflow (Italian "Assegnatario"
  overlaps the avatar in the Terminal and Plan previews).
- Terminal theme in Hindi: monospaced spaces look wide.
- E-mails show comments as escaped text; wiki markup is not rendered.
- No e-mail for work logs or deleted comments (webhooks do send them).
- `solstice user passwd` (command line) is not written to the audit log.
- Import warnings stay in French in the setup wizard.
- Boards: no backlog column on Kanban boards; drag and drop does not work by
  touch (the card menu is used on phones); swimlane collapse buttons come late
  in the tab order; sprints cannot be edited before they start nor deleted;
  completing a sprint cannot create the next one.
- Editor: a `[` inside monospace text, in a table cell that is not the last of
  its row, cannot be typed.
- REST API not implemented: createmeta and editmeta, issue picker, remote
  links, editing or deleting work logs, changing the reporter, time tracking
  writes, `/rest/auth/1/session`, groups, create/update/delete of versions,
  components, filters and sprints.

## 1.1

- Dashboards and agile reports (burndown, velocity, cumulative flow, sprint
  report).
- Bulk edit, transition, move and delete; clone; move between projects;
  keyboard shortcuts; configurable columns; CSV import.
- LDAP/Active Directory with group sync; OpenID Connect single sign-on.
- Usability pass across the product.
