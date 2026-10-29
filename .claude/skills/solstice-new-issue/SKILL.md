---
name: solstice-new-issue
description: Turn a problem or idea I describe into a GitHub issue of mflr0/Solstice, in the format the solstice-issue skill works from. Use when I ask to write, file or open a ticket or issue for Solstice.
argument-hint: "<problem, in my words>"
---

Write one issue of mflr0/Solstice from the problem in `$ARGUMENTS` (or the conversation), so that the `solstice-issue` skill can pick and deliver it without asking me. Don't fix the problem here.

Every call re-reads the whole context, so make few of them: send independent calls in the same turn.

## 1. Understand

1. If `$ARGUMENTS` and the conversation say nothing about the problem, ask me in one line and stop.
2. In the same turn:
   - Look for duplicates: `gh issue list --state all --limit 20 --search "<2-3 keywords>" --json number,title,state` and `gh pr list --state open --search "<keywords>" --json number,title`.
   - Open trackers: `gh issue list --state open --limit 200 --json number,title,body -q '.[]|select(.body|test("- \\[[ x]\\] #"))|"\(.number) \(.title)"'`.
   - Read the code the problem touches (`grep -n`, line ranges, several files per call) in the main checkout. Read only: don't edit, don't switch branches.
3. A duplicate is open: show it and stop. A closed one: say so and go on only if the problem is back.
4. Find the facts the ticket rests on: where it happens (`file:line`), what the user sees, numbers where they help (sizes, counts, the failing input). Separate what you verified from what you believe. Don't guess a cause you haven't seen in the code: say it is unknown.

## 2. Settle

If the problem leaves real choices open (what the expected behaviour is, how far the scope goes, one ticket or several), grill me with the `grilling` skill, with your recommendation for each. Otherwise go on.

Several independent changes are several issues. Things that must happen in order get `Waits for #n` lines.

## 3. Draft

English, whatever language I write in. Short sentences, plain words, no filler.

**Title**: `<Area>: <the problem or change>`, sentence case, no trailing period (e.g. `Issue view: estimate fields cut their placeholder`). The area is the screen or the backend part a user or a developer would name.

**Body**, in this order, blank line between blocks:

1. The problem: what happens, where, for whom, with the `file:line` and numbers from step 1.4. One short paragraph.
2. The change: what must be true once done. Say how only when there is one obvious way, or a constraint the worker can't see (keep an `aria-label`, don't add a migration).
3. `In scope: <path>, <path>.` One line, paths from the repo root, separated by `, `, ending with a period. A directory ends with `/`. Add `if …` only after a path that may not be needed (`frontend/src/i18n/*.ts if the key goes`). `pick.sh` splits this line to find tickets that share files with open bugs, so keep it exact.
4. `Out of scope: …` only when a neighbour is tempting (other modals, webhooks).
5. `Pins: …` the tests that must keep passing, or the new test that proves the change (name the test and what it asserts). For a bug, the test that fails before the fix.
6. `Proof: …` the checks the worker runs:
   - backend: `the server checks: \`(cd backend && test -z "$(gofmt -l .)" && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go test ./...)\``, plus `the integration tests in \`backend/tests\`` if it touches the API or the database;
   - frontend: `the interface checks \`(cd frontend && pnpm install --frozen-lockfile && pnpm format:check && pnpm lint && pnpm test && pnpm build)\``;
   - plus the UI check, concrete: which page, which role, which languages, widths (1280 px, 390 px), skins, and what to measure.
7. `Stop and say so in the ticket if: …` the conditions under which the worker must not just go on: a migration, a file outside the scope, an existing test needing another expectation, a design choice the ticket didn't settle.
8. `Waits for #n.` one line per open issue or unmerged PR that must land first.
9. `Tracked in #N.` if it belongs to an open tracker.

Leave out a block that has nothing to say, except the problem, the change, `In scope` and `Proof`.

**Labels**, from `gh label list` only:
- kind, one: `bug` (something is broken), `enhancement` (new feature), `polish` (small visual or wording fix), `refactor`, `hardening`, `documentation`;
- areas, every one that applies: `backend`, `frontend`, `i18n`, `themes`, `accessibility`, `security`, `ci`, `import`.
- Never `in progress`: it is the claim lock of `solstice-issue`.

**Security**: the repo is public. For a vulnerability, write only the area and the class of problem in the issue (`Details are with the maintainer.`), and give me the details in the chat.

## 4. Confirm and create

1. Show me the title, labels, tracker, and the body as it will be posted, in one message. Wait for my go. Apply what I change.
2. Write the body to a file in the scratchpad, then: `gh issue create --repo mflr0/Solstice --title "<title>" --label "<a>,<b>" --body-file <file>`.
3. With a tracker: append `- [ ] #<new>` to its list where the order says (end of the list by default), with `gh issue view <N> --json body -q .body > <file>`, edit the file, `gh issue edit <N> --body-file <file>`. Don't touch the rest of the tracker.
4. If the new issue must land before an existing one, add `Waits for #<new>.` to that one (before its `Tracked in` line) the same way.
5. Reply with the issue URL, one line.
