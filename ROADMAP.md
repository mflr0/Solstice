# Solstice roadmap

Solstice does not aim to match Jira feature for feature. It aims to make
leaving Jira safer, faster and more faithful than any other way, and to make
leaving Solstice just as easy.

The detailed list of what remains before 1.0 is in `todo.md`.

## 1.0

- **Jira audit without migrating.** `solstice import -check` reads a Jira
  database and reports what a migration would bring and what it would lose,
  without writing anything. Useful to any Jira administrator, even one who
  never migrates.
- **Anonymous diagnostic.** The same report without any content (counts,
  schema shapes, errors), safe to attach to a public issue. It gives real
  feedback on real instances without anyone sharing their data.
- **Every Jira version, with messy data.** The synthetic Jira generator
  covers every major Jira version whose distribution can be downloaded, and
  the quirks that old instances accumulate. The import is tested against
  each of them.
- **Migration guide.** Dry run, parallel run, cutover, rollback, and keeping
  the domain name so that `/browse/KEY-1` links keep working.

## 1.x

- **Delta import.** Import while Jira is still running, then only the
  changes at cutover, so that a large instance can switch in minutes.
- **Jira XML backups.** Import the `entities.xml` and `activeobjects.xml` of
  a Jira backup archive, for those who cannot reach the database. To check:
  which instances and exports produce them.
- **Common plugins.** Import the data of the most widespread plugins from
  their `AO_*` tables, starting with Tempo's worklog attributes.
- **Full export.** Everything Solstice holds, in a documented open format
  (JSON and attachments), at any time.
- **Commits on issues.** Link commits and merge requests that mention an
  issue key, from GitHub, GitLab and Gitea webhooks.
- **PostgreSQL for Solstice itself**, not only as an import source. After the
  refactoring of `internal/api` into a service layer (see `todo.md`), so the
  SQL is changed once.

## Project health

- **Release policy.** Semantic versions, security fixes for the previous
  minor version, and written upgrade guarantees.
- **Resource budget.** Measure and publish the smallest machine that runs a
  100,000-issue instance, and check it in CI.
