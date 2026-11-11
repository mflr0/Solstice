---
name: solstice-issue
description: Pick, claim and deliver one ticket of the Solstice tracker (project SOL on solstice.suolumi.fr), coordinated with the other Claude sessions.
disable-model-invocation: true
argument-hint: "[SOL-N] [label or type...]"
---

Work one ticket of the SOL project from pick to merge. Tickets live on https://solstice.suolumi.fr, handled with the `mcp__solstice__*` tools. Code and pull requests live on GitHub, mflr0/Solstice. This skill holds the whole workflow, including what CLAUDE.md and memory also say. `pick.sh` sits next to this file and does the git and pull-request steps.

Every call re-reads the whole context, so make few of them: send independent calls in the same turn, and chain dependent shell steps with `&&` in one Bash call.

`SOL-N` is the ticket, `N` its number: ports and record names below use `N`. Comments and descriptions are Jira wiki markup: `{{code}}`, `{code}…{code}`, `[text|url]`, `*bold*`.

## 1. Pick

1. In one turn, two `search_issues` calls:
   - Candidates: `project = SOL AND statusCategory != Done AND issuetype != Epic ORDER BY Rank ASC`, fields `summary, issuetype, status, labels, issuelinks, Epic Link`, `maxResults` 200.
   - Open bugs: `project = SOL AND issuetype = Bug AND statusCategory != Done`, fields `description`.
2. With `SOL-N` in `$ARGUMENTS`, check that ticket only. Otherwise keep the tickets in **To Do**. Leave out Stories unless `story` or `enhancement` is a filter. Other words narrow the list to tickets with that label or type.
3. A ticket is **blocked** when an inward `is blocked by` link points to an issue whose status category isn't Done, or, unless it is a Bug, when its `In scope` line shares a path with an open bug's.
4. **In Progress** tickets are claimed by another session. One is **stale**, and counts as To Do in its place, when both are true:
   - its last status change to In Progress, from `get_issue` with `expand: ["changelog"]`, is more than 24 h old;
   - `pick.sh --last-commit SOL-N` prints nothing, or a time more than 24 h old.
   Check all of them in one turn.
5. If nothing is free, say in one line how many are blocked and claimed, and stop: no per-ticket table.
6. Otherwise `get_issue` the first free one, fields `summary, issuetype, labels, description, comment, issuelinks, Epic Link`, and check its `In scope` against the bugs' now. Read it for dependencies links can't show:
   - Prose ("do the small-fixes ticket first", "after the typed-confirmation ticket", "Parked: needs … first"). Find the ticket they name.
   - Its "Out of scope" and "Stop and say so" lines, for tickets that go first.

If one names an open ticket, link it with `link_issues` (type `Blocks`, `inwardIssue` the ticket that goes first, `outwardIssue` this one), so the next pick catches it. A ticket with any open dependency is out, even if only part of it depends on it. Don't scope it down: take the next free candidate. If none is left, say so in one line and stop.

## 2. Claim

1. `get_transitions` on the ticket. If none leads to In Progress, another session holds it: go back to step 1 with the next candidate. The status is the lock: don't message the other sessions. A stale claim: first take the transition back to To Do, with a comment `Stale claim, taken over.`.
2. `transition_issue` with the one leading to In Progress.
3. Choose a short `<topic>` and run `pick.sh --start <topic>`. It checks the main checkout has a local git identity, fetches and creates the worktree `../Solstice-<topic>` on a new branch `v1.0/<topic>` from `origin/feat/v1.0`. No local identity: ask me, the global one is not for this repo.

If the work is dropped at any point, move the ticket back to To Do.

## 3. Plan

- Before planning, read the code each "Stop and say so" condition points to, to see whether it is already met.
- In the same first batch of reads, check the ticket's premises: each "In scope" path exists, and the data the change needs already reaches the code that draws it (e.g. a field the API must send). A wrong premise found now costs one call; found mid-edit, several.
- A design linked as an Artifact: don't read the Artifact itself (its type instructions alone are thousands of tokens). Read the one artboard the ticket names, directly by `path` (`project/<Name>.dc.html`); list the files first only if the name isn't clear. Then `grep -n` the saved file for the part you need.
- If the ticket leaves real design choices open (names, where code lives, sites outside the scope list, a stop condition already met), grill me with the `grilling` skill before editing. If a stop condition triggered it, comment the outcome on the ticket with `add_comment`.
- Otherwise, state the plan in two lines and start.
- Ignore stop conditions about files under open pull requests.
- Before trusting a claim that something is unused or dead, grep the code for it.

## 4. Implement

1. Work only in the worktree `--start` created, never in the main checkout: other sessions use it.
2. Read with `grep -n` and line ranges, several files per call, rather than one file per call.
3. Make the change. Comments: a one-line doc comment on exported identifiers; otherwise only when the why isn't obvious.
4. Run the proof the ticket names ("Proof that nothing moved", "Pins"). By default:
   - Server: `(cd backend && test -z "$(gofmt -l .)" && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go test ./...)`
   - Interface: `(cd frontend && pnpm install --frozen-lockfile && pnpm format:check && pnpm lint && pnpm test && pnpm build)`
5. Integration tests: run them myself, against a separate database pointed to by `SOLSTICE_TEST_DB`, never the dev `solstice` database. If auto mode blocks that (or `make e2e`), don't work around it: for a backend-only change, rely on the CI integration job and say so; otherwise ask me to run `! make e2e`.
6. For a new test, see it fail with the code it covers broken, then restore the code.
7. If a "Stop and say so in the ticket if" condition is met while editing, stop editing. Grill me with the `grilling` skill on what triggered it, with your recommendation. Then comment the outcome on the ticket with `add_comment`.
8. If the change needs a UI check (see step 5.1), do it yourself first; see "UI check" below.

### UI check

Start your own servers from the worktree and drive them with the Playwright MCP tools (`mcp__playwright__*`, headless, one isolated browser per session). Other sessions do the same at the same time, so:

- **Never use my dev server on 8080**: no requests, no setup, no checks, no cleanup. Whenever you need a server to test, set up or clean up, start your own as below, even for a single request. The one exception is migrations.

- **Ports from the ticket number**, never 8080 (mine) or another session's: server on `18000+N`, Vite on `19000+N`. Check first with `lsof -iTCP:<port> -sTCP:LISTEN`. A listener whose `lsof -p <pid> -a -d cwd` is your worktree is your own leftover: stop it with `kill <pid>`. Anything else: add 1000 and check again.
- **Stopping a server: never `pkill`, `killall`, or anything else that picks processes by name or pattern.** They run on my whole machine, next to other sessions: a `pkill` once took down Docker, my desktop apps and every session's servers. Start each server with the Bash tool's `run_in_background`, even for a single request, never with `&`, `nohup` or a subshell, so `TaskStop` can stop it. Without a task ID, use `kill <pid>` with one PID from `lsof -iTCP:<port> -sTCP:LISTEN`, after checking with `lsof -p <pid> -a -d cwd` that its directory is your worktree or its `backend`/`frontend`. Never `kill -9`, and never a PID you didn't check. If something unexpected dies, stop and tell me.
- **Server**, in the background, from `<worktree>/backend` once the interface checks have built `dist`:
  `SOLSTICE_CONFIG=<main checkout>/backend/solstice.conf SOLSTICE_DATA=<main checkout>/backend/data SOLSTICE_HTTP_ADDR=127.0.0.1:<18000+N> go run ./cmd/solstice serve`
  Wait for `/healthz` to answer 200.
- **Interface**, in the background, from `<worktree>/frontend`:
  `SOLSTICE_BACKEND=http://127.0.0.1:<18000+N> pnpm dev --host 127.0.0.1 --port <19000+N> --strictPort`
  Browse `http://127.0.0.1:<19000+N>`: Vite serves the worktree's sources, the server its API.
- **Shared database.** Every server uses the dev database. If it is down, start it with `docker compose -f <main checkout>/compose.yaml up -d`, always that file so every session shares the same containers and volumes. Only if the Docker daemon itself isn't running, ask me. If the branch adds a migration, don't run `migrate` on it: say so and ask. Background jobs (mail, webhooks) run in every server, so don't judge mail delivery from this setup.
- **Shared data.** Don't change instance settings, or the preferences (language, theme) of the mock users: other sessions are logged in as them. Logging in as them changes nothing shared. Name any record you create `check #<N> …` (users `check<N>-…`), and delete it once nobody needs it any more (users: deactivate): right after testing, or, if you leave me something to check, at merge time. Don't edit records you didn't create: to test an edit, create the record first.
- **Logging in**: on `/login`, fill the first text input and the password input, then press Enter in the password input. `seed` / `seed-dev-only` is the admin and has no language, so it follows the browser.
- **Other languages, other roles**: in `browser_run_code_unsafe`, open a second context with `page.context().browser().newContext({ locale: 'fr-FR' })` and log in there as the mock user of that language (`marie` fr, `alice` en, …). The mock users aren't admins, so they also show the read-only views. Close the context in a `finally`. A theme check needs a throwaway user.
- **Setting up data**: use the admin API on your own server, e.g. `curl -u seed:seed-dev-only -H 'X-Solstice: 1' -H 'Content-Type: application/json' -X POST http://127.0.0.1:<18000+N>/api/admin/…`. Read the endpoint from `frontend/src/lib/admin-api.ts`.
- **Failure paths**: make the request fail with `page.route(...)` in `mcp__playwright__browser_run_code_unsafe` (e.g. `route.fulfill({ status: 500, … })`), then `page.unrouteAll()`. The same works to fake a response the database can't easily give (a long error, a failed delivery).
- **Prove the check can fail**: for a behaviour fix, undo the fix in the worktree (Vite reloads it), run the same check and see it fail, then restore the fix and check `git diff` is back to what it was.
- **What to read**: `browser_snapshot` or `browser_find` for text, including screen-reader-only text; `browser_evaluate` for attributes and computed styles (measure: `scrollWidth > clientWidth` means clipped; a component's own scoped CSS can change what a shared class does); `browser_console_messages` for errors; a screenshot only when layout matters.
- **One pass, compact output**: the tool echoes the whole script back, so cover every combination the ticket asks for (users, languages, widths, skins) in one `browser_run_code_unsafe` loop rather than one call each. Return one short line per combination (`alice 1280 carnet: ok`) and details only for what failed, never indented JSON.
- **Screenshots**: at most one per look you must judge, clipped to the element's `boundingBox()` plus a margin, never at fixed page coordinates (scrolling moves it). Save them in the scratchpad and read them.
- **What it finds**: fix it in the worktree with a new commit, and say so in the report.
- **When done testing**, right away, not at cleanup: close the browser. Then:
  - If something is left for me to check in the UI (step 5.3), keep the server and Vite you started running, and the records I need to look at.
  - Otherwise delete the records and list them to check they are gone (deleting a board leaves its `Board check #<N> …` filter), then stop the servers (`TaskStop`, then check the ports with `lsof`).
  - Either way, leave the Docker containers (database, Mailpit) running.

The `playwright` MCP server is pinned (`@playwright/mcp@0.0.83 --browser chromium`) to the Chromium in `~/Library/Caches/ms-playwright`, and writes its files to `~/Library/Caches/playwright-mcp-output`. If a `.playwright-mcp/` folder shows up in a checkout, delete it. If the Playwright tools are missing, or the browser won't start, don't install anything: tell me and fall back to listing the checks for me.

## 5. Deliver

1. Choose the path:
   - **Needs a UI check** (it touches the frontend, or changes something the user sees that no test pins): `draft`.
   - **Nothing to check in the UI** (backend-only, covered by unit tests and CI integration tests): `ready`.
2. Commit and ship in one Bash call: `git -C <worktree> add -A && git -C <worktree> commit -q -m "<subject>" && pick.sh --ship SOL-N <topic> draft|ready`.
   - Commit once the work is done and verified. Split it into several commits only for several meaningful steps, and then pass `--title "<one-line summary>"`. Subject line only, no body, in the conventional-commit style of `git log` (e.g. `test(jirafixture): …`).
   - `--ship` pushes the branch, opens the pull request against `feat/v1.0` with the commit subject as title and an empty description, and comments `Closes SOL-N.` on it. The stale-claim check finds the pull request through this comment. It doesn't close the ticket: GitHub doesn't know SOL.
   - Then `add_remote_link` on the ticket: `object` `{"url": "<PR URL>", "title": "PR #<PR>"}`, `globalId` the PR URL, `relationship` `"implemented in"`.
   - Pass `--note "<text>"` to add to that comment any file changed outside the ticket's scope, and why.
3. Then:
   - **Draft**: report what the UI check (step 4.8) verified, check by check, and what it found. Then ask me to test only what the browser couldn't judge (how it looks in a theme, a real screen reader). Give me the interface URL (`http://127.0.0.1:<19000+N>`), the login to use, and a direct link to each page to open, with the records to look at. Wait for my answer. If your UI check covered everything and nothing is left for me, go to step 4 without asking.
   - **Ready**: go to step 4 without asking.
4. Merge, once it is ready, once I approve the draft ("good", "all good", "continue"; don't ask whether to merge), or once nothing is left for me to check. Stop any server of the UI check still running and delete the records it left, then run `pick.sh --finish <PR> <topic>` with the Bash timeout at 600000. It marks a draft ready, waits for every CI check, merges the commit they ran on, removes the worktree and the local and remote branch, and pulls `feat/v1.0` in the main checkout.
   - Once it merged: `transition_issue` to Done, with `fields` `{"resolution": {"name": "Done"}}` and `update` `{"comment": [{"add": {"body": "Done in [PR #<PR>|<PR URL>]."}}]}`. If that was the last open ticket of its epic, say so in the report: I close epics.
   - Exit 4: a check failed, or was still pending. Fix it with a new commit, push it (`git -C <worktree> push`), and run `--finish` again; or stop and tell me.
   - If it stops on anything else, fix the cause and run it again: it resumes after what is already done.
5. Report in a few lines: the ticket, the PR, what the change does, what was verified, and anything left open. On a "continue", go back to step 1 for the next ticket.

Once a branch is pushed, never `commit --amend` + force-push: append commits.
