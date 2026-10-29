---
name: solstice-new-issue
description: Turn a problem or idea I describe into GitHub issues of mflr0/Solstice (one ticket, or an epic with small tickets), in the format the solstice-issue skill works from. Use when I ask to write, file or open a ticket or issue for Solstice.
argument-hint: "<problem, in my words>"
---

Write the issues of mflr0/Solstice for the problem in `$ARGUMENTS` (or the conversation), so that the `solstice-issue` skill can pick and deliver them without asking me. Don't fix the problem here.

Every call re-reads the whole context, so make few of them: send independent calls in the same turn.

## 1. Investigate

1. If `$ARGUMENTS` and the conversation say nothing about the problem, ask me in one line and stop.
2. Read the code on the latest base, not the main checkout's files: `git fetch -q origin feat/v1.0`, then `git grep -n <pattern> origin/feat/v1.0 -- <paths>` and `git show origin/feat/v1.0:<path> | sed -n '<a>,<b>p'`, several per call. Don't edit, don't switch branches.
3. Check the problem is still there on `origin/feat/v1.0`. If it is gone, say so with the commit or code that fixed it, and stop.
4. Find the facts the ticket rests on: where it happens (`file:line`), what the user sees, numbers where they help (sizes, counts, the failing input). Separate what you verified from what you believe. Don't guess a cause you haven't seen in the code: say it is unknown.
5. In the same turn as the reads, list the open trackers: `gh issue list --state open --limit 200 --json number,title,body -q '.[]|select(.body|test("- \\[[ x]\\] #"))|"\(.number) \(.title)"'`.

## 2. Settle

If the problem leaves real choices open (what the expected behaviour is, how far the change goes), grill me with the `grilling` skill, with your recommendation for each. Otherwise go on without asking.

Then choose the shape:
- **One ticket** when one pull request of reasonable size solves it.
- **An epic** when it needs several pull requests, or holds independent changes: one tracker issue and one small ticket per change. Each ticket is one pull request a session can finish alone. Order them so the foundations land first, and give a ticket a `Waits for #n` line only when it truly can't start before #n is merged.
  - A ticket also waits for one whose change it relies on to come out right, even with no file in common. For each ticket, ask what its result depends on that another ticket changes, and read that code on `origin/feat/v1.0`. Example: converting px to rem renders the same only once the shared-styles ticket sets `:root` to `font-size: 100%`. Until then, 1rem is the skin's `--font-size`, so the component and skin tickets wait for it.

## 3. Draft

English, whatever language I write in. Short sentences, plain words, no filler.

The aim is a better application, not a fence. The scope says where the work is expected; it doesn't forbid fixing a real problem the worker finds next to it.

### Ticket

**Title**: `<Area>: <the problem or change>`, sentence case, no trailing period (e.g. `Issue view: estimate fields cut their placeholder`). The area is the screen or the backend part a user or a developer would name.

**Body**, in this order, blank line between blocks:

1. The problem: what happens, where, for whom, with the `file:line` and numbers from step 1.4. One short paragraph.
2. The change: what must be true once done. Say how only when there is one obvious way, or a constraint the worker can't see (keep an `aria-label`, don't add a migration).
3. `In scope: <path>, <path>.` One line, paths from the repo root, separated by `, `, ending with a period. A directory ends with `/`. Add `if …` only after a path that may not be needed (`frontend/src/i18n/*.ts if the key goes`). `pick.sh` splits this line to find tickets that share files with open bugs, so keep it exact. List where the work is expected; it is not a limit.
4. Then, one line: `Related problems found on the way: fix them here if they are small and on these files, otherwise file a ticket.`
5. `Out of scope: …` only for work another ticket already covers (name it), never to keep a real problem out.
6. `Pins: …` the tests that must keep passing, or the new test that proves the change (name the test and what it asserts). For a bug, the test that fails before the fix.
7. `Proof: …` the checks the worker runs:
   - backend: `the server checks: \`(cd backend && test -z "$(gofmt -l .)" && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go test ./...)\``, plus `the integration tests in \`backend/tests\`` if it touches the API or the database;
   - frontend: `the interface checks \`(cd frontend && pnpm install --frozen-lockfile && pnpm format:check && pnpm lint && pnpm test && pnpm build)\``;
   - plus the UI check, concrete: which page, which role, which languages, widths (1280 px, 390 px), skins, and what to measure.
8. `Stop and say so in the ticket if: …` only for decisions the worker shouldn't take alone: a migration, a change of behaviour users rely on, a design choice the ticket didn't settle. Not for touching a file outside the scope.
9. `Waits for #n.` one line per open issue or unmerged PR that must land first.
10. `Tracked in #N.` if it belongs to an epic or an open tracker.

Leave out a block that has nothing to say, except the problem, the change, `In scope` and `Proof`.

**Labels**, from `gh label list` only:
- kind, one: `bug` (something is broken), `enhancement` (new feature), `polish` (small visual or wording fix), `refactor`, `hardening`, `documentation`;
- areas, every one that applies: `backend`, `frontend`, `i18n`, `themes`, `accessibility`, `security`, `ci`, `import`.
- Never `in progress`: it is the claim lock of `solstice-issue`.

### Epic

**Title**: the goal, e.g. `Usability pass across the product`. Labels: the kind and areas of its tickets.

**Body**: what the epic is for and what it covers, in a few lines; what it leaves to other issues (name them); then `Tickets in the order they should be done:` and one `- [ ] #n` line per ticket. `pick.sh` orders tickets by their position in this list.

### Security

The repo is public. For a vulnerability, write only the area and the class of problem in the issue (`Details are with the maintainer.`), and give me the details in the chat.

## 4. Create

Don't wait for my go. Write each body to a file in the scratchpad and create with `gh issue create --repo mflr0/Solstice --title "<title>" --label "<a>,<b>" --body-file <file>`, which prints the URL (the number is its last part).

- **One ticket**: create it. If it belongs to an open tracker, append `- [ ] #<new>` to that tracker's list (end of the list unless the order says otherwise): `gh issue view <N> --json body -q .body > <file>`, edit the file, `gh issue edit <N> --body-file <file>`. Don't touch the rest of the tracker.
- **Epic**: create the epic first with an empty list, so the tickets can carry `Tracked in #<epic>.`. Create the tickets in order, so each `Waits for` names a number that exists. Then write the list into the epic with `gh issue edit`.
- If a new ticket must land before an existing one, add `Waits for #<new>.` to that one (before its `Tracked in` line) the same way.

Reply with the URLs, one line each, the epic first, and one line on anything you couldn't verify.
