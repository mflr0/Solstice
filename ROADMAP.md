# Solstice roadmap

Solstice does not aim to match Jira feature for feature. It aims to make
leaving Jira safer, faster and more faithful than any other way, and to make
leaving Solstice just as easy.

The detailed list of what remains before 1.0 is in `todo.md`.

## 1.0

- **Jira audit without migrating.** `solstice import -check` reads a Jira
  database and reports what a migration would bring and what it would lose,
  without writing anything. Useful to any Jira administrator, even one who
  never migrates. Done (#26).
- **Anonymous diagnostic.** The same report without any content (counts,
  schema shapes, errors), safe to attach to a public issue. It gives real
  feedback on real instances without anyone sharing their data. Done (#561).
- **Every Jira version, with messy data.** The synthetic Jira generator
  covers every major Jira version whose distribution can be downloaded, and
  the quirks that old instances accumulate. The import is tested against
  each of them. Done (#27).
- **Migration guide.** Dry run, parallel run, cutover, rollback, and keeping
  the domain name so that `/browse/KEY-1` links keep working. Done (#25,
  `docs/en/migration.md`).

Also in 1.0, ahead of the plan: dashboards and agile reports, bulk edit,
clone and move, keyboard shortcuts, configurable columns, CSV import,
LDAP/Active Directory and OpenID Connect sign-in. In progress: an MCP server
with scoped tokens and OAuth (#484), and theme packages (#526 to #529).

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
- **Other sources.** Import from other issue trackers (Redmine, OpenProject,
  Huly and others), on the same model as the Jira import: read the source,
  report, then import.
- **PostgreSQL for Solstice itself**, not only as an import source.

## Project health

- **Release policy.** Semantic versions, security fixes for the previous
  minor version, and written upgrade guarantees.
- **Resource budget.** Measure and publish the smallest machine that runs a
  100,000-issue instance, and check it in CI.
