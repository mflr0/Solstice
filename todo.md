# Solstice: what remains

Status on 2026-10-19. Work happens on `feat/v1.0` (CI green: vet, staticcheck,
gosec and tests for the server; format, lint, tests and build for the
interface; the API tests against a seeded MariaDB; govulncheck and
`pnpm audit`). `main` is protected (pull request and green CI required, no
force push) and has not been merged yet: it still holds the first version plus the
current README, code of conduct and license, and it does not compile. Merging
`feat/v1.0` into `main` is the maintainer's decision.

## Working conventions

- Commits: plain messages, no `Co-Authored-By` line.
- Every change goes on a `v1.0/<topic>` branch and a pull request to
  `feat/v1.0`; the maintainer merges by hand.
- Data: tests and agents use only the synthetic data of
  `backend/cmd/jira-fixture`, never real data (rule 6 of CODE_OF_CONDUCT.md).
- Integration tests live in `backend/tests`.
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

The goal of 1.0 is to be the safest way to leave Jira, not to match its
features (see `ROADMAP.md`). The release waits for items 5 and 6.

1. Check the administration end to end in the interface against
   `backend/internal/api/ADMIN_API.md` (checked against the code in pull
   request #81). Also check the project settings tabs (general, components,
   versions, notifications), the instance name and banner in the app shell,
   and the "update available" badge.
2. Integrated updates: run the root helper in a systemd container (update
   succeeds, bad checksum refused, unhealthy version rolled back).
3. Translations: the meaning review is done (pull request #94). Left to a
   decision or a native speaker: the Hindi link labels ("is blocked by",
   "blocks", "duplicates", "is cloned by"), one French word for assignee
   ("Assigné" or "Responsable", which is also the project lead) and
   "courriel" or "e-mail", the German link verbs ("durch" or "von"), the
   Italian security level ("Riservatezza" or "livello di sicurezza").
4. A contact address for conduct reports in CODE_OF_CONDUCT.md. The
   documentation is written in English and French (`docs/en`, `docs/fr`,
   pull requests #85 and #91).
5. Anonymous Jira audit: `solstice import -check` writes the report; a
   `-anonymous` option should make it hold no content at all (counts,
   schema shapes, errors only), so anyone can attach it to an issue.
6. Jira versions: `jira-fixture` generates the quirks of old instances
   (`Options.Messy`), but from one entity model only. Generate databases for
   every major Jira version whose distribution can be downloaded (7.x to the
   latest), each read from its own entity model, and run
   `scripts/e2e-import.sh` against each.
7. Every admin object reachable from the menus: the Updates page exists
   (`frontend/src/ui/admin/UpdatesPage.svelte`) and is linked from the
   system page, but `AdminArea.svelte` has no route for `/admin/updates`.
   Then a pass over the whole interface for other pages with no menu entry
   or link.
8. Release: the maintainer generates the Ed25519 key pair and puts the
   public key in `deploy/linux/update.sh` (see `scripts/release.sh`; until
   then the update helper refuses every update), privacy audit of the whole
   history, pull request from `feat/v1.0` to `main`, tag `v1.0.0`, GitHub
   release with the archives and `SHA256SUMS` (the updater needs them), then
   a one-line installer.

## Hardening (from an OWASP Top 10:2025 review of the code, done on 2026-10-16)

No critical or high finding. The review of the access control, injection,
cryptography, configuration and logging paths found nothing exploitable
without an account. Done since: more audit events, the supply-chain pins and
checks, resource limits, the plain-HTTP warning of the setup wizard, and
`?tls=` in the database backup of `update.sh`. Remaining, not blocking:

- A password change by the user: in one transaction (`updateMe` sets the
  password and ends the other sessions in separate statements,
  `internal/api/auth.go:191-203`), and revoke the personal access tokens,
  which it keeps. The change by an administrator revokes the reset links,
  not the personal tokens.
- Smaller items are tracked outside this file until fixed.

## Refactoring (after 1.0, or with a freeze on `internal/api`)

`internal/api` has 58 files (tests aside) mixing HTTP, business rules and
SQL, and the issue logic likely exists twice: `issues.go` for the interface,
and `rest_issues.go` / `rest_write.go` for the REST compatibility layer.
Extract a shared service layer, one domain per pull request (issues, then
boards, then projects), starting with an issue. The structural tickets of the
2026-10-18 audit come first (#113, `docs/dev/refactor-audit-2026-10-18.md`).

## Known bugs and polish

- Account menu (all themes): capped at 340 px by the generic `.popover`
  rule in `frontend/src/app.css`, which overrides `.account-menu`'s own
  max-height; the theme list is cut and Mode, Language and Sign out are below
  the fold. Only Luna and Retro override it.
- Orbite, light mode: account menu text is near-white on the white menu (it
  inherits the top bar's ink colour, `themes/orbite.css`).
- Retro: on pages using SettingsLayout, the eyebrow and title are white on
  the grey window body and an empty title bar is drawn above them (the
  title-bar style only matches `.page > .page-head`, `themes/retro.css`).
- Classique: the NOVA scrum board's last column is cut at 1440 px, with no
  sign that the board scrolls sideways.
- Crayon: hand-drawn glyphs overhang their boxes by 3 to 5 px (nav links,
  status stamp, count badges); anything with `overflow: hidden` clips them.
- Carnet: home page tables stacked one above the other have unaligned
  columns; the orange and red priority glyphs have low contrast on paper.
- After a few quick reloads, pages can wait about 10 s on "Loading…":
  likely stale `/api/events` streams using up the browser's 6 connections
  per host (`frontend/src/lib/live.svelte.ts`); check the server closes the
  stream when the client goes away.
- Saved filters created with a board are named in French in every language
  ("Tableau …", `internal/api/boards.go:1339`).
- Search results table: the summary column collapses to about 22 px around
  1024 px wide, and to about 10 characters at 1440 px in the Terminal theme.
- Visual pass not done yet: 390 px and 1024 px widths, backlog and
  administration pages in the non-default themes.
- The "More" filter popover (320 px) opens partly off-screen up to at least
  1440 px (45 to 109 px past the edge depending on the theme, page scrolls
  sideways): it is always left-aligned (`ui/search/FilterPopover.svelte`).
- `/projects/KEY/settings` scrolls horizontally at 390 px.
- Top bar navigation is clipped at 390 px.
- Theme gallery previews: long labels overflow (Italian "Assegnatario"
  overlaps the avatar in the Terminal and Plan previews).
- Terminal theme in Hindi: monospaced spaces look wide.
- E-mails show comments as escaped text; wiki markup is not rendered.
- No e-mail for work logs or deleted comments (webhooks do send them).
- The import report and warnings are in English only (not translated).
- `internal/api/issues.go:1395` ignores the error of reading the `type` field.
- Boards: no backlog column on Kanban boards; drag and drop does not work by
  touch (the card menu is used on phones); swimlane collapse buttons come late
  in the tab order; deleting a board leaves its sprints with no board.
- Editor: a `[` inside monospace text, in a table cell that is not the last of
  its row, cannot be typed.
- REST API not implemented: createmeta and editmeta, issue picker, remote
  links, `/rest/auth/1/session`, groups, create/update/delete of versions,
  components and filters, creating sprints (`POST /rest/agile/1.0/sprint` is
  not registered), changing the reporter (refused by design).

## 1.1

- Dashboards and agile reports (burndown, velocity, cumulative flow, sprint
  report).
- Bulk edit, transition, move and delete; clone; move between projects;
  keyboard shortcuts; configurable columns; CSV import.
- LDAP/Active Directory with group sync; OpenID Connect single sign-on.
- Usability pass across the product.
