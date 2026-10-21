---
name: solstice-issue
description: Pick, claim and deliver one Solstice GitHub issue, coordinated with the other Claude sessions.
disable-model-invocation: true
argument-hint: "[number] [label...]"
---

Work one issue of mflr0/Solstice from pick to merge. This skill holds the whole workflow, including what CLAUDE.md and memory also say. `pick.sh` sits next to this file.

## 1. Pick

Run `pick.sh $ARGUMENTS`.
- With a number, it checks that ticket only, whatever its assignee.
- Without one, it lists open, unassigned tickets. It leaves out `enhancement` unless that is a filter, and other label words narrow the list.
- Order: position in the ticket's "Tracked in #N" issue, then issue number.
- It flags as blocked: tickets with a "Waits for #n" (in the body or on the tracker line) whose #n is an open issue or an unmerged PR, and tickets other than bugs that share an "In scope" file with an open bug. It lists tickets claimed by another session apart.
- If nothing is free, it prints one line (`Nothing free: …`). Relay that line and stop: no per-ticket table.

Read only the first free candidate, body and comments, for dependencies the script can't see:
- Prose ("do the small-fixes ticket first", "after the typed-confirmation ticket", "Parked: needs … first"). Find the ticket they name.
- Its "Out of scope" and "Stop and say so" lines, for tickets that go first.

If one names an open ticket or an unmerged PR, add it to the ticket's "Waits for #n" line (or add that line before "Tracked in") with `gh issue edit`, so `pick.sh` catches it next time. A ticket with any open dependency is out, even if only part of it depends on it. Don't scope it down: take the next free candidate. If none is left, say so in one line and stop.

## 2. Claim

1. Get this session's short ref from `ListAgents`: the bracketed id after its name (e.g. `0dd017`). Session names can contain spaces, which the claim comment can't hold.
2. Run `pick.sh --claim N <session>`. Exit 3 means another session holds or just won the ticket: go back to step 1 with the next candidate. The label is the lock: don't message the other sessions.

If the work is dropped at any point, run `pick.sh --release N`.

## 3. Plan

- Before planning, read the code each "Stop and say so" condition points to, to see whether it is already met.
- If the ticket leaves real design choices open (names, where code lives, sites outside the scope list, a stop condition already met), grill me with the `grilling` skill before editing. If a stop condition triggered it, comment the outcome on the ticket with `gh issue comment`.
- Otherwise, state the plan in two lines and start.
- Ignore stop conditions about files under open pull requests.
- Before trusting a claim that something is unused or dead, grep the code for it.

## 4. Implement

1. Check `git config --local user.email` in the main checkout (the first line of `git worktree list`; worktrees share its local config). If it is empty, ask me: the global identity is not the one for this repo.
2. `git fetch origin`, then create the worktree next to the repo, on a new branch from `feat/v1.0` (not `main`):
   `git worktree add -b v1.0/<topic> ../Solstice-<topic> origin/feat/v1.0`
   Work only in that worktree, never in the main checkout: other sessions use it.
3. Make the change. Comments: a one-line doc comment on exported identifiers; otherwise only when the why isn't obvious.
4. Run the proof the ticket names ("Proof that nothing moved", "Pins"). By default:
   - Server: `(cd backend && test -z "$(gofmt -l .)" && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go test ./...)`
   - Interface: `(cd frontend && pnpm install --frozen-lockfile && pnpm format:check && pnpm lint && pnpm test && pnpm build)`
5. Integration tests: run them myself, against a separate database pointed to by `SOLSTICE_TEST_DB`, never the dev `solstice` database. If auto mode blocks that (or `make e2e`), don't work around it: for a backend-only change, rely on the CI integration job and say so; otherwise ask me to run `! make e2e`.
6. For a new test, see it fail with the code it covers broken, then restore the code.
7. If a "Stop and say so in the ticket if" condition is met while editing, stop editing. Grill me with the `grilling` skill on what triggered it, with your recommendation. Then comment the outcome on the ticket with `gh issue comment`.
8. If the change needs a UI check (see step 5.4), do it yourself first; see "UI check" below.

### UI check

Start your own servers from the worktree and drive them with the Playwright MCP tools (`mcp__playwright__*`, headless, one isolated browser per session). Other sessions do the same at the same time, so:

- **Ports from the ticket number**, never 8080 (mine) or another session's: server on `18000+N`, Vite on `19000+N`. Check first with `lsof -iTCP:<port> -sTCP:LISTEN`. A listener whose `lsof -p <pid> -a -d cwd` is your worktree is your own leftover: stop it. Anything else: add 1000 and check again.
- **Server**, in the background, from `<worktree>/backend` once the interface checks have built `dist`:
  `SOLSTICE_CONFIG=<main checkout>/backend/solstice.conf SOLSTICE_DATA=<main checkout>/backend/data SOLSTICE_HTTP_ADDR=127.0.0.1:<18000+N> go run ./cmd/solstice serve`
  Wait for `/healthz` to answer 200.
- **Interface**, in the background, from `<worktree>/frontend`:
  `SOLSTICE_BACKEND=http://127.0.0.1:<18000+N> pnpm dev --host 127.0.0.1 --port <19000+N> --strictPort`
  Browse `http://127.0.0.1:<19000+N>`: Vite serves the worktree's sources, the server its API.
- **Shared database.** Every server uses the dev database. If it is down, don't start Docker Compose: ask me. If the branch adds a migration, don't run `migrate` on it: say so and ask. Background jobs (mail, webhooks) run in every server, so don't judge mail delivery from this setup.
- **Shared data.** Don't change instance settings, or the preferences (language, theme) of the mock users: other sessions are logged in as them. Logging in as them changes nothing shared. Name any record you create `check #<N> …` (users `check<N>-…`), and delete it afterwards (users: deactivate). Don't edit records you didn't create: to test an edit, create the record first.
- **Logging in**: on `/login`, fill the first text input and the password input, then press Enter in the password input. `seed` / `seed-dev-only` is the admin and has no language, so it follows the browser.
- **Other languages, other roles**: in `browser_run_code_unsafe`, open a second context with `page.context().browser().newContext({ locale: 'fr-FR' })` and log in there as the mock user of that language (`marie` fr, `alice` en, …). The mock users aren't admins, so they also show the read-only views. Close the context in a `finally`. A theme check needs a throwaway user.
- **Setting up data**: use the admin API on your own server, e.g. `curl -u seed:seed-dev-only -H 'X-Solstice: 1' -H 'Content-Type: application/json' -X POST http://127.0.0.1:<18000+N>/api/admin/…`. Read the endpoint from `frontend/src/lib/admin-api.ts`.
- **Failure paths**: make the request fail with `page.route(...)` in `mcp__playwright__browser_run_code_unsafe` (e.g. `route.fulfill({ status: 500, … })`), then `page.unrouteAll()`. The same works to fake a response the database can't easily give (a long error, a failed delivery).
- **Prove the check can fail**: for a behaviour fix, undo the fix in the worktree (Vite reloads it), run the same check and see it fail, then restore the fix and check `git diff` is back to what it was.
- **What to read**: `browser_snapshot` or `browser_find` for text, including screen-reader-only text; `browser_evaluate` for attributes and computed styles (measure: `scrollWidth > clientWidth` means clipped; a component's own scoped CSS can change what a shared class does); `browser_console_messages` for errors; a screenshot only when layout matters.
- **What it finds**: fix it in the worktree with a new commit, and say so in the report.
- **When done testing**, right away, not at cleanup: stop the server and Vite you started (`TaskStop`, then check the ports with `lsof`) and close the browser. Leave the Docker containers (database, Mailpit) running.

The `playwright` MCP server is pinned (`@playwright/mcp@0.0.83 --browser chromium`) to the Chromium in `~/Library/Caches/ms-playwright`, and writes its files to `~/Library/Caches/playwright-mcp-output`. If a `.playwright-mcp/` folder shows up in a checkout, delete it. If the Playwright tools are missing, or the browser won't start, don't install anything: tell me and fall back to listing the checks for me.

## 5. Deliver

1. Commit once the work is done and verified. Split it into several commits only for several meaningful steps. Subject line only, no body, in the conventional-commit style of `git log` (e.g. `test(jirafixture): …`).
2. Push the branch to `origin` and open the pull request against `feat/v1.0`. Title: the commit subject (several commits: a one-line summary in the same style). Empty description (`--body ""`). Draft or ready: step 4 decides.
3. Comment `Closes #N.` on the pull request, adding any file changed outside the ticket's scope and why. The stale-claim check finds the pull request through this comment. The issue doesn't close by itself: step 6 closes it.
4. Choose the path:
   - **Needs a UI check** (it touches the frontend, or changes something the user sees that no test pins): open it as a draft (`--draft`). Report what the UI check (step 4.8) verified, check by check, and what it found. Then ask me to test only what the browser couldn't judge (how it looks in a theme, a real screen reader), or say there is nothing left. Wait for my answer.
   - **Nothing to check in the UI** (backend-only, covered by unit tests and CI integration tests): open it ready for review and go to step 5 without asking.
5. Merge, once it is ready or once I approve the draft ("good", "all good", "continue"; don't ask whether to merge):
   - Mark a draft ready (`gh pr ready`).
   - Wait for every CI check (`gh pr checks <PR> --watch`). If one fails, fix it with a new commit, or stop and tell me.
   - `gh pr merge <PR> --merge --delete-branch`.
6. Run `pick.sh --done N <PR>`. It removes the label, deletes the "Claimed by session …" comments, closes the issue and ticks it in its tracker.
7. Clean up: stop any server of the UI check still running, remove the worktree (`git worktree remove ../Solstice-<topic>`), the local branch and the remote branch, whichever are still there. Check with `git worktree list`, `git branch` and `git ls-remote --heads origin <branch>`.
8. In the main checkout: `git checkout feat/v1.0 && git pull`.
9. Report in a few lines: the ticket, the PR, what the change does, what was verified, and anything left open. On a "continue", go back to step 1 for the next ticket.

Once a branch is pushed, never `commit --amend` + force-push: append commits.
