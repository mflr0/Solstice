# Migrating from Jira

Solstice can take over a Jira Server or Data Center instance. It reads Jira's
database directly, so read access is enough, and it never writes to Jira. You
can import as many times as you want, test on a copy, and go back to Jira if
you change your mind.

This page covers what the import needs, how to run it, how to plan a
migration, and what comes across and what does not. Installing Solstice is in
[install.md](install.md), backups in [backups.md](backups.md).

## Before you start

**Supported sources.**

- Jira Server or Data Center, version 7 or later. Solstice checks the schema,
  not a version number: a database without the `schemepermissions.permission_key`
  column is refused with a message asking you to upgrade Jira first.
- A Jira database on PostgreSQL (`postgres://…`) or MySQL (`mysql://…`).
  Other engines are not supported as a source.
- Solstice itself runs on MariaDB.

**What you need.**

- The address, name, user and password of the Jira database. A user that can
  read it is enough.
- The Jira server's time zone (Jira, Administration, System info). MySQL stores
  dates without a time zone, and dates without a time (due dates, versions)
  depend on it with both engines. A wrong value shifts those dates.
- Optionally, the Jira home directory, readable from the machine where Solstice
  runs, to copy attachment files. Without it, only the attachments' metadata
  is imported.

**For a consistent result**, stop Jira first, or import from a copy of its
database. Importing from a database that people are still writing to can give
an inconsistent result.

**The import replaces everything in Solstice.** There is no merge. Before any
import over an instance that holds data, back up its database and its data
directory: that is the only way back. See [backups.md](backups.md).

## Running the import

There are three ways. They all run the same import.

### The command line

```sh
solstice import \
  -from postgres://jira:secret@db-jira.example.org:5432/jira \
  -tz Europe/Paris \
  -jira-home /var/atlassian/application-data/jira
```

| Option | Meaning |
|---|---|
| `-from` | The Jira database: `postgres://user:pass@host/db` or `mysql://user:pass@host/db`. Required. |
| `-tz` | The Jira server's time zone, for example `Europe/Paris`. Required. |
| `-jira-home` | The Jira home directory, to copy attachment files. Optional. |
| `-replace` | Wipe Solstice's data before importing. Without it, the import refuses to run on a database that already holds users or projects. |
| `-json` | Print the report as JSON instead of text. |

The command reads the same configuration as the server (`SOLSTICE_DB_URL`,
`SOLSTICE_DATA`…) and applies pending database migrations first. Attachment
files are copied into `<data>/attachments`.

Percent-encode special characters in the passwords of the URLs (`@` as
`%40`). The only options accepted after `?` are `tls` (`true`, `false`,
`skip-verify`, `preferred`), `timeout`, `readTimeout` and `charset` for MySQL,
and `sslmode` and `connect_timeout` for PostgreSQL. Any other option is
refused.

The report lists, for each kind of record, how many were imported and how many
were skipped (orphaned references), then a "Check" list of things that deserve
a look. Read it.

### The setup wizard

On a new installation, the setup wizard at `/setup` can import a Jira instance
instead of creating an administrator. You enter the Jira database address, the
time zone and, optionally, the Jira home directory. A test button connects to
the database and shows the number of projects, issues and users it found, and
whether the Jira home holds attachments. Then the import runs and shows its
progress. Afterwards, the wizard asks who administers Solstice: you pick an
imported account that can sign in (administrators from Jira come first), or
you create a new administrator.

The Jira home must be an absolute path that exists on the machine where
Solstice runs.

### The administration page

An administrator can import from Administration, Jira import, on a running
instance. It takes the same three values as the wizard and a test button. To
start, you must tick that you have a backup, and type the name of Solstice's
database.

While the import runs, Solstice is in maintenance. Under `/api/` and `/rest/`,
only three requests still answer: `GET /api/settings/public`, and, for the
administrator who started the import, `GET /api/admin/import` (the progress)
and `GET /api/me`. Every other request answers 503. Keep the progress page
open. When it finishes, reload.

Only one import runs at a time per database, whether started from here, from
the wizard or from the command line.

What an import keeps of Solstice's own configuration, from the administration
and from `-replace` alike:

- Kept as is: instance settings (general, e-mail including the SMTP password,
  audit retention), webhooks (their delivery log is emptied) and the audit log.
- Kept for accounts that exist after the import, matched by user name:
  personal access tokens, notification preferences, interface language, last
  sign-in, open sessions and administrator rights.
- Accounts created in Solstice whose user name Jira does not have are kept,
  with their password, but without groups or roles.
- The administrator who starts the import from the administration keeps their
  account, password, rights and session, even when Jira has an account of the
  same name.
- Replaced: everything that comes from Jira, namely users, groups, projects,
  issues and what they hold, catalogues, custom fields, filters, boards and
  sprints. Filter subscriptions, project notification settings, e-mails waiting
  to be sent and webhook deliveries go with them.

## What is imported

Jira's own identifiers are kept (issue keys and numbers, board, sprint and
filter ids, attachment ids), so links between records survive and an import
can be run again.

| Area | What comes across |
|---|---|
| Users | One account per Jira user, from the directory with the highest priority. Display name, e-mail address, active or not. Users deleted from Jira but still named in issues are kept as inactive accounts. |
| Passwords | Internal-directory passwords are kept as Jira hashes (`{PKCS5S2}`) and converted to Solstice's own hash at the person's first sign-in. People keep their password. |
| Groups | Groups and their members. Nested groups are flattened: a person belongs to the groups of the group they are in. |
| Administrators | Members of the groups that hold Jira's Administer or System Administer permission become Solstice administrators. |
| Application access | Accounts that are not in a licensed group (and are not administrators) are imported inactive, as Jira would refuse them. |
| Projects | Key, name, description, lead, category, former keys of renamed projects, the issue types each project offers, components, versions. |
| Roles | Project roles and who holds them in each project. |
| Permissions | Each project gets its own copy of its permission scheme, and of its issue security scheme (levels and members). |
| Catalogues | Issue types, statuses and their categories, priorities, resolutions, link types. |
| Workflows | Each project gets its own copy of the workflows its scheme uses: statuses, transitions, initial status, and which issue types follow which workflow. |
| Custom fields | Definitions, options (including cascading levels), the projects and issue types they apply to, and all their values. |
| Issues | All fields, sub-tasks, epics, links, remote links, labels, watchers, votes, security levels, the old keys of moved issues. |
| Comments and work logs | With their visibility restrictions to groups and roles. |
| Attachments | Metadata always. Files when `-jira-home` is given. |
| History | The full change history of each issue. |
| Filters | Saved filters with their JQL, shares and favourites. |
| Jira Software | Boards with their columns, quick filters and swimlanes, sprints and which issues were in them, board ranking, epic links. |

Rules to know:

- **Permissions are never widened.** What Solstice cannot express (permissions
  granted through a custom field, or to a service desk portal, for example) is
  removed, and the report says how many. A confidential issue never becomes
  visible to more people than before. The same goes for filter shares.
- **Workflow behavior is not carried over.** Jira conditions, validators and
  post-functions are Java code. Solstice applies its own rules instead:
  permissions decide who can make a transition, and an issue reaching a done
  status without a resolution gets the default resolution.
- **A project without a permission scheme** is imported with no access for
  anyone. The report names it. Fix it in its settings.

## What is not imported

- Dashboards and gadgets.
- Notification schemes. Solstice has its own notification settings: each
  person chooses their events, and each project can add recipients.
- Screens, field configurations and the other schemes that decide which field
  is shown or required where. Solstice shows the fields that apply to the
  project and issue type.
- Jira Service Management data, and the data of other plugins. Only the Jira
  Software boards, sprints, ranking and epics are read.
- Passwords of accounts in an external directory (LDAP and similar), or stored
  in an old format. Those accounts are imported without a password. The
  report counts the active ones. Turn on LDAP sign-in (see
  [administration](administration.md#ldap-and-active-directory)), set one
  with `solstice user passwd -username NAME`, or send a reset link from the
  administration.
- Attachment files, when `-jira-home` is not given. The report says so. Run
  the import again with `-jira-home`, or accept that those files are missing:
  their entries answer 404 on download.
- Jira's server configuration: mail server, application links, the license.
  Set the mail server in Administration, Email.

Custom fields whose Jira type Solstice cannot edit are imported and shown, but
not editable ("Jira type, display only").

## Planning the migration

### 1. Dry run on a copy

Do not start with your production Jira. Restore a copy of its database on a
separate database server, install Solstice on a test machine (see
[install.md](install.md)), and import from the copy:

```sh
solstice import -from postgres://jira:secret@db-copy.example.org:5432/jira \
  -tz Europe/Paris -jira-home /mnt/jira-home-copy -replace
```

Check the report. Open projects, issues with long histories, boards and
sprints, restricted comments, and a few confidential issues as someone who must
not see them. Check that people can sign in. Time the import, so that you know
how long the cutover needs.

You can repeat this as often as needed. Each run with `-replace` starts again
from Jira's data.

### 2. Parallel run

While people keep working in Jira, keep your test instance and let a few
people try it. There is no incremental import: each import replaces
everything, so anything entered in the test instance is lost at the next
import, except what Solstice keeps (see above). Tell testers so.

Use the time to settle the choices that Solstice does not carry over: set up
the mail server, review the permissions of key projects, set the public
address, and decide who is an administrator.

### 3. Cutover day

1. Announce the freeze. Stop Jira, or at least stop people from writing to
   it, so that the database is not changing.
2. Back up Jira's database and, if you copy attachments, make sure its home
   directory is complete. Do not change them again.
3. On the production Solstice, back up its database and data directory
   ([backups.md](backups.md)).
4. Run the import with `-replace` and `-jira-home`, from the command line or
   the administration page.
5. Read the report. Deal with the "Check" list: accounts without a password,
   projects without access, missing attachment files.
6. Check a handful of projects, boards and issues, and sign in with an
   ordinary account.
7. Point your domain name at Solstice (next section) and announce the move.

### Keeping the same domain name

Issue pages in Solstice use Jira's address form. Solstice serves
`/browse/KEY-1`, so links in bookmarks, wikis, chats and old e-mails keep
working when the domain name points at Solstice. An old project key resolves
too: if a project was renamed in Jira, `/browse/OLD-12` still finds the issue.
Other addresses that keep working:

- `/projects/KEY` opens the project.
- `/secure/RapidBoard.jspa?rapidView=ID` opens the board with that id (with
  `view=planning`, its backlog), and `/secure/RapidView.jspa?rapidView=ID`
  opens its settings. Board ids are kept by the import.
- `/rest/api/2/…` and `/rest/agile/1.0/…`: see [rest-api.md](rest-api.md).
- `/secure/attachment/{id}/{filename}`, the address of attachment files.

Other Jira pages are not served, such as dashboards and administration pages
(`/secure/…` apart from the two boards above). Solstice answers its own "not
found" page.

To keep the domain name:

1. Set the public address in Solstice to the domain name
   ([administration.md](administration.md)), and put it behind HTTPS
   ([reverse-proxy.md](reverse-proxy.md)).
2. On cutover day, once the import is checked, change the DNS record or the
   proxy so that the name leads to Solstice. If Jira was at a path such as
   `/jira`, redirect that path.
3. Lower the DNS time to live a day or two before, so that the change spreads
   fast.

Scripts that call the REST API with a token need a new Solstice token: tokens
are not imported. See [rest-api.md](rest-api.md).

### Rollback

Solstice never writes to the Jira database. As long as you keep Jira's
database and home directory untouched, going back is: point the domain name
back at Jira and start it.

What you cannot bring back is the work done in Solstice after the cutover:
there is no export to Jira. Decide a date after which you no longer go back,
and keep Jira stopped but intact until then.

If the cutover failed in Solstice itself (a failed or partial import), restore
Solstice's database and data directory from the backup of step 3, or run the
import again with `-replace`. Even when an import fails, Solstice puts back
its own kept data (see above), so the administrator who started it can sign in
and try again.

## Troubleshooting

- "the Solstice database already holds data; use -replace to overwrite it":
  the target is not empty. Add `-replace` if you mean it.
- "another import into this database is running": wait for it to finish.
- "this Jira version is too old": upgrade Jira to 7 or later, then import.
- A connection error that names an option: the database URL carries an option
  Solstice does not accept (see the list above).
- Dates are an hour or a day off: the `-tz` value is not the Jira server's
  time zone.
