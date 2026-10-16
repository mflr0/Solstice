# Solstice: what remains

Status on 2026-10-16. Work happens on `feat/v1.0` (CI green: vet, staticcheck,
gosec and tests for the server; format, lint, tests and build for the
interface). `main` is protected (pull request and green CI required, no force
push) and has not been merged yet: it still holds the first version plus the
current README, code of conduct and license, and it does not compile. Merging
`feat/v1.0` into `main` is the maintainer's decision.

## Working conventions

- Commits: plain messages, no `Co-Authored-By` line.
- Data: tests and agents use only the synthetic data of
  `backend/cmd/jira-fixture`, never real data (rule 6 of CODE_OF_CONDUCT.md).
- Legal: no code, assets, fonts or styles from Atlassian or Microsoft, and no
  brand names in themes. Jira is named only for migration, API compatibility
  and the trademark note.
- Design: no generic "AI" look (no purple gradients, glassmorphism, Inter
  font, emojis).
- Languages: English is the source (`frontend/src/i18n/en.ts`, Go string
  literals); French, German, Italian and Hindi are translations. A new text
  needs all five. Translations from cheap models need a meaning review: about
  3% of the server messages they translated had lost their meaning.
- Large changes start with an issue (rule 7 of CODE_OF_CONDUCT.md).
- Delegating to opencode worked well with: one git worktree per task on an
  `oc/*` branch, a prompt with fixed rules, a starting check, exact commands
  and expected outputs, a report file that is not committed, and a review of
  every branch before cherry-picking it.

## Remaining for 1.0, in order

1. Check the administration end to end against
   `backend/internal/api/ADMIN_API.md`. It was written by agents that were
   interrupted and never reviewed; a deadlock was already found and fixed
   there. Also check the project settings tabs (general, components,
   versions, notifications), the instance name and banner in the app shell,
   and the "update available" badge.
2. Run the API tests in CI. They need an imported database
   (`SOLSTICE_TEST_DB`), today built from a Jira schema file that is not
   versioned. Write a Go seeder for a synthetic Solstice database and run the
   tests against a MariaDB service in the workflow. Needed before any large
   refactoring.
3. Review the administration and update code (an automated review was
   started and stopped): permissions of every admin route, `go test -race`,
   SQL, the root update helper, deletions and the replacing import.
4. Security review: XSS (wiki rendering, editor), CSRF, security headers
   (CSP, framing, referrer), sessions, uploads. Login throttling counts
   failures per client address; behind a reverse proxy every client has the
   proxy's address, so five failures lock everyone out for a minute: support
   a trusted proxy header.
5. Integrated updates: run the root helper in a systemd container (update
   succeeds, bad checksum refused, unhealthy version rolled back).
6. Meaning review of the interface translations (about 520 texts per
   language) with a strong model; three Hindi link labels ("is blocked by"…)
   need a native speaker.
7. The setup wizard catalogue (`frontend/src/i18n/setup/`) still has French
   as its source: switch it to English like the rest.
8. AGPL: a "Source code" link in the interface. Third-party notices: the
   fonts (OFL-1.1) ship with the application and their license must come
   with them.
9. Documentation in English and French: installation, administration,
   backups and restore, upgrades, reverse proxy (examples in
   `deploy/proxy/`), REST API, `SECURITY.md`. A contact address for conduct
   reports in CODE_OF_CONDUCT.md.
10. Release: privacy audit of the whole history, pull request from
    `feat/v1.0` to `main`, tag `v1.0.0`, GitHub release with the archives and
    `SHA256SUMS` (the updater needs them), then a one-line installer.

## Refactoring (after 1.0, or with a freeze on `internal/api`)

`internal/api` has 41 files mixing HTTP, business rules and SQL, and the
issue logic likely exists twice: `issues.go` for the interface, and
`rest_issues.go` / `rest_write.go` for the REST compatibility layer. Extract a
shared service layer, one domain per pull request (issues, then boards, then
projects), after item 2 above, starting with an issue.

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
- The import report and warnings are in English only (not translated).
- `internal/api/issues.go:1338` ignores the error of reading the `type` field.
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
