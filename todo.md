# Solstice: what remains

Status on 2026-10-30. Work happens on `feat/v1.0` (CI green: vet, staticcheck,
gosec and tests for the server; format, lint, tests and build for the
interface; the API tests against a seeded MariaDB; govulncheck and
`pnpm audit`). `main` is protected (pull request and green CI required, no
force push). `feat/v1.0` was merged into it once (pull request #409, on
2026-10-23); no version is tagged yet.

## Working conventions

- Commits: plain messages, no `Co-Authored-By` line.
- Every change goes on a `v1.0/<topic>` branch and a pull request to
  `feat/v1.0`; the maintainer merges by hand.
- Data: tests use only the synthetic data of `backend/cmd/jira-fixture`,
  never real data (rule 6 of CODE_OF_CONDUCT.md).
- Integration tests live in `backend/tests`.
- Legal: no code, assets, fonts or styles from Atlassian or Microsoft, and no
  brand names in themes. Jira is named only for migration, API compatibility
  and the trademark note.
- Design: no purple gradients, glassmorphism, Inter font or emojis.
- Languages: English is the source (`frontend/src/i18n/en.ts`, Go string
  literals); French, German, Italian and Hindi are translations. A new text
  needs all five, each reviewed for its meaning, not only its words.
- Large changes start with an issue (rule 7 of CODE_OF_CONDUCT.md).

## Remaining for 1.0, in order

The goal of 1.0 is to be the safest way to leave Jira, not to match its
features (see `ROADMAP.md`). The release (#29) waits for item 1.

1. Anonymous Jira audit: `solstice import -check -anonymous`, a report with
   no content at all (counts, schema shapes, errors only), so anyone can
   attach it to an issue (#561).
2. Translations: one word for assignee and e-mail in French, and for the
   security level in Italian (#563). Left to a native speaker: the Hindi link
   labels ("is blocked by", "blocks", "duplicates", "is cloned by") and the
   German link verbs ("ist blockiert durch", "ist dupliziert von").
3. Release (#29): the maintainer generates the Ed25519 key pair and puts the
   public key in `deploy/linux/update.sh` (see `scripts/release.sh`; it still
   holds the placeholder, so the update helper refuses every update), privacy
   audit of the whole history, pull request from `feat/v1.0` to `main`, tag
   `v1.0.0`, GitHub release with the archives and `SHA256SUMS` (the updater
   needs them), then a one-line installer.

## Open work

Each epic lists its tickets in the order they should be done.

- MCP server with scoped tokens and OAuth (#484): the tools are done; left
  are the OAuth authorization server (#492) and dynamic client registration
  with the admin view of grants (#493).
- Theme packages: built-in themes load from packages (#526), admins import,
  export and manage them (#527), users submit themes for approval (#528), and
  a theme authoring kit (#529).
- Highlight what searches match (#517), starting with one every-word match
  rule for list filters (#518).
- REST: create a sprint through `POST /rest/agile/1.0/sprint` (#562).
- Smaller tickets: future dates show as "just now" (#510), the project
  switcher and a search without filters (#513), the theme menu keeps the mode
  (#524), the new issue form remembers the last project (#525), a click
  outside a modal closes it (#551).

## Hardening

The OWASP Top 10:2025 review of 2026-10-16 found no critical or high issue,
and its findings are fixed (#30 to #33, #50, #74 to #78, #114 to #122).
Smaller items are tracked as issues until fixed.

## Refactoring

No service layer will be extracted from `internal/api` (#34, closed): the
REST layer calls the interface handlers through `uiCall` instead of copying
the issue logic, and the 2026-10-18 audit
(`docs/dev/refactor-audit-2026-10-18.md`) advised against splitting the
package. Its local findings were handled by the #113 train.

## Known bugs and polish

Tracked as GitHub issues with the `bug` and `polish` labels. The list that
stood here on 2026-10-19 is fixed (#35 to #58).
