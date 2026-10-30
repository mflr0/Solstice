# Administration guide

This page is for Solstice administrators. It follows the menu of the
administration (`/admin`), which only accounts with the administrator right
can open. It says what each part does and what to watch for. It does not
describe every button.

Installation, the reverse proxy, backups and upgrades are in
[install.md](install.md), [reverse-proxy.md](reverse-proxy.md),
[backups.md](backups.md) and [upgrades.md](upgrades.md).

What lives where:

- The configuration file and environment variables only bootstrap the
  server: database URL, data directory, listen address, and LDAP and OpenID
  Connect sign-in. See
  `solstice -h`.
- Everything else described below is stored in the database and applies at once,
  without a restart.
- Every change made in the administration is written to the audit log.

## General

### Overview

Lists what is left to set up (no project yet, e-mail turned off) and links to
every section.

### General settings

| Setting | Notes |
|---|---|
| Instance name | 100 characters at most. Shown next to the logo. |
| Public address | The address people open Solstice at: `https://solstice.example.org`, without a path. Used for links in e-mails. An `https://` address makes session cookies `Secure`. |
| Time zone | An IANA name such as `Europe/Paris`. Used for dates typed in searches, hours in e-mails and daily digests. Empty means the server's own zone. |
| Default language | For people who chose none, and for the e-mails they receive. `en`, `fr`, `de`, `it` or `hi`. |
| Session length | 1 to 365 days, 30 by default. Applies to future sign-ins. |
| Minimum password length | 8 to 128, 12 by default. Applies to new passwords; existing ones keep working. |

Watch for:

- The public address and the time zone can first come from
  `SOLSTICE_HTTP_BASEURL` and `SOLSTICE_TZ`. Once saved here, the value in the
  database wins. "Restore the original value" removes it and goes back to the
  file, the environment or the default.
- Without a public address, e-mails go out without links, and invitations and
  password reset links cannot be sent.

### Announcement banner

A message at the top of every page, for everyone. Up to 500 characters, with
the tone "information" or "warning". Anyone can hide it for themselves; it
comes back if the text changes. Use it for planned maintenance.

## Users and groups

### Users

Every person who signs in has an account. The list shows the first 200
matches: narrow the search to find the others.

- **User names** are lower case: letters, digits and `. _ @ -`, starting with
  a letter or a digit.
- **Creating an account.** Either choose a password (at least the minimum
  length), or send an invitation by e-mail. An invitation needs an e-mail
  address on the account, working e-mail and a public address. The person gets
  a link, valid for 7 days, to choose their own password. "Resend invitation"
  replaces the link.
- **Deactivating** an account closes its sessions at once and stops sign-ins.
  Its issues, comments and history stay. You can reactivate it later.
- **Administrator right.** Gives access to all of the administration. You
  cannot remove your own right.
- **Changing a password.** Three ways: type one, generate one (shown once,
  pass it on at once), or send the person a link by e-mail. The link works
  once and for one hour; the password does not change until it is used.
  Typing or generating a password closes the person's sessions.
- **Groups.** The account dialog sets which groups a person belongs to.
- People can also ask for a reset link themselves from the sign-in page, when
  e-mail links are available.
- Accounts imported from a Jira external directory (LDAP and similar) have no
  password. With LDAP sign-in on, they use their directory password (see
  below). Otherwise send them a link, or set one.

If every administrator is locked out, create or repair an account from the
server's command line. The commands run against the configured database and
are written to the audit log:

```sh
solstice user add -username admin2 -name "Second Admin" -admin
solstice user passwd -username admin2
```

Without `-password`, a password is generated and printed.

### Groups

Groups gather people to give them permissions together. They are used in
project permissions, in project roles, to restrict comments and work logs, to
share filters, and in the access token policy. Group names are stored in lower
case and are unique.

The Groups page lists every group with its description and its number of
members. A search box filters by name or description. "New group" asks for a
name and a description, 255 characters at most each. Open a group to rename
it, change its description or manage its members.

- **Members.** Once a group exists, its dialog has a picker to add people and
  a list to remove them. Unlike the name and the description, members are
  added and removed at once, without "Save". People can also be put in groups
  from the account dialog (see Users).
- **Deleting** asks for confirmation. The group disappears from the
  permissions, roles and security levels that use it; its members keep their
  accounts.

Watch for:

- Deleting a group removes its grants, role memberships and security level
  memberships. It is refused while comments or work logs are restricted to the
  group, because deleting it would hide them from everyone. The dialog then says
  how many comments and work logs still depend on the group.
- Renaming a group keeps the comments and work logs restricted to it.

### LDAP and Active Directory

People can sign in with their LDAP or Active Directory password. It is set in
the configuration file, not in the administration: see the `SOLSTICE_LDAP_*`
keys in [installation](install.md). Restart Solstice after a change. Example
for Active Directory:

```sh
SOLSTICE_LDAP_URL=ldaps://dc1.example.org
SOLSTICE_LDAP_BINDDN=CN=solstice,OU=Services,DC=example,DC=org
SOLSTICE_LDAP_BINDPASSWORD=service-account-password
SOLSTICE_LDAP_USERBASE=OU=People,DC=example,DC=org
SOLSTICE_LDAP_USERFILTER=(sAMAccountName={username})
SOLSTICE_LDAP_GROUPFILTER=(member:1.2.840.113556.1.4.1941:={dn})
SOLSTICE_LDAP_REQUIREGROUP=solstice-users
```

The group filter above includes nested groups (Active Directory only). For
OpenLDAP, the default `(member={dn})` reads groups of type `groupOfNames`.

At each sign-in, Solstice searches the person with the service account, checks
the password by signing in to the directory as them, then reads their groups.

- **Which accounts.** An account marked "LDAP" in Users signs in through the
  directory only. A local account with a password keeps it, even if the
  directory knows the same name: administrators created in Solstice still get
  in when the directory is down. An existing account without a password, such
  as one imported from Jira's external directory, becomes an LDAP account at
  its first LDAP sign-in. A name no account has gets a new LDAP account.
- **Name and address** are copied from the directory at each sign-in, when it
  has them.
- **Groups.** The person is put in the Solstice groups named like their
  directory groups (in lower case); missing groups are created. Those groups
  are then the directory's: each sign-in removes the person from the ones
  they left. Groups that do not come from the directory are never changed.
- **Administrators.** With `SOLSTICE_LDAP_ADMINGROUP`, each sign-in grants or
  removes the administrator right of LDAP accounts. Without it, the right is
  set in Solstice only.
- **Passwords.** Solstice neither sets nor resets the password of an LDAP
  account: the password actions are hidden, and the reset link is not sent.

Watch for:

- Removing someone from the directory, or from the required group, refuses
  their next sign-in. Sessions and access tokens already open keep working
  until they expire: deactivate the account to end them at once.
- Without `SOLSTICE_LDAP_URL`, LDAP accounts cannot sign in.
- Groups and the administrator right set by hand on an LDAP account are
  replaced at the next sign-in when the directory decides them.

### Single sign-on (OpenID Connect)

People can sign in with an OpenID Connect provider (Keycloak, Microsoft Entra
ID, Authentik, Dex, Google…): the sign-in page then shows a "Sign in with …"
button above the password form. It is set in the configuration file: see the
`SOLSTICE_OIDC_*` keys in [installation](install.md). Restart Solstice after a
change.

At the provider, register Solstice as a confidential client using the
authorization code flow, with the redirect URI
`<public address>/api/auth/oidc/callback`, for example
`https://solstice.example.org/api/auth/oidc/callback`. The public address must
be set (Administration → General): without it, the button is hidden. Example
for Keycloak:

```sh
SOLSTICE_OIDC_ISSUER=https://id.example.org/realms/staff
SOLSTICE_OIDC_CLIENTID=solstice
SOLSTICE_OIDC_CLIENTSECRET=client-secret
SOLSTICE_OIDC_NAME=Example ID
SOLSTICE_OIDC_REQUIREGROUP=solstice-users
```

Groups come from the claim `SOLSTICE_OIDC_GROUPSCLAIM` (`groups` by default),
which most providers send only when asked: add the scope or the mapper your
provider needs (Keycloak: a "Group Membership" mapper; Entra ID: "groups claim"
in the token configuration; some providers: the `groups` scope, in
`SOLSTICE_OIDC_SCOPES`).

- **Which accounts.** The first sign-in finds the account whose user name is
  the claim `SOLSTICE_OIDC_USERNAMECLAIM` (`preferred_username` by default;
  Entra ID: `upn` or `email`), then links it to the person's identifier at the
  provider: later sign-ins find it by that identifier, even if the user name
  changes at the provider. An LDAP account, or one without a password (such as
  one imported from Jira's external directory), is linked; a name no account
  has gets a new account, marked "SSO" in Users. A local account with a
  password is never linked: the person is told to sign in with the password.
  An LDAP account linked this way signs in either way.
- **Name and address** are copied from the provider at each sign-in, when it
  sends them. An address the provider marks as not verified is not taken.
- **Groups and administrators** work as for LDAP (see above), with
  `SOLSTICE_OIDC_REQUIREGROUP` and `SOLSTICE_OIDC_ADMINGROUP`.
- **Passwords.** An SSO account has no password in Solstice: its e-mail
  address is set by the provider, and the password actions are hidden.

Watch for:

- Signing out of Solstice does not sign out of the provider: the next "Sign in
  with …" may go through without asking.
- Removing someone at the provider, or from the required group, refuses their
  next sign-in only, as with LDAP: deactivate the account to end its sessions
  at once.
- Changing `SOLSTICE_OIDC_ISSUER` unlinks every account: each is linked again
  by user name at its next sign-in.
- Failed sign-ins are in the audit log (method `oidc`), and the server log says
  why.

### Roles

Project roles are shared by all projects. Who holds a role is set for each
project, in the project settings, under "Roles in this project". A role can
then be given permissions in that project, like a group.

The Roles page lists the roles with their description. You can create a role,
change its name and description, and delete it. Only the role itself is
managed here; its holders are chosen project by project.

Watch for:

- Deleting a role removes it from every project. It is refused while comments
  or work logs are restricted to it, and the dialog says how many.

## Projects

### Projects page

Lists every project with its key, lead, category, issue count and state, and
links to each project's settings.

- **Creating a project.** The key is 2 to 10 capital letters or digits,
  starting with a letter. It cannot be changed afterwards, and it must differ
  from every current and former key. You also choose the starting workflow
  (Software: To Do, In Progress, In Review, Done; or Simple: three statuses),
  the lead (you by default), who can work in the project (only the lead,
  everyone with an account, a group or a role), the issue types and the
  default assignee. The lead gets every permission. The first project of a
  fresh instance also fills the empty catalogues with defaults, in the
  instance's language.
- **Archiving** makes a project read-only and removes it from the project
  lists and pickers. Nothing is lost, and you can unarchive it at any time.
  Prefer it to deleting.
- **Deleting** is permanent: issues, comments, attachments and their files,
  history, components, versions, workflows and permissions all go. A project
  with issues asks you to type its key. Boards and saved filters stay, and
  may show nothing.
- **Categories** group projects in the project list. Deleting a category leaves
  its projects without one.

### Project settings

Each project has its own rules. Nothing is shared through schemes, so a change
touches no other project. Only the project's administrators (and global
administrators) can open the settings. The page has six tabs, in this order:
General, Components, Versions, Permissions and roles, Workflows and
Notifications. It opens on General. Changes in General, Notifications and
Workflows wait for "Save", and the page asks before you leave a tab with
unsaved changes. Components and Versions act at once, from their dialogs.

**General.** The project's name, description, lead, category, default assignee
and issue types. The key cannot be changed.

- The lead has every right in the project.
- The default assignee, for issues created without one, is nobody or the
  project lead.
- The issue types are the ones people can choose in this project; at least one
  must stay. A type that issues use cannot be removed: the save is refused and
  lists each such type with its number of issues. Change the type of those
  issues first.

**Components.** Components split a project into parts, such as "API" or
"Billing". An issue can belong to one or several of them. A component has a
name (unique in the project, whatever the case), a description, a lead and a
rule for who new issues are assigned to: the project's default, the
component's lead, the project lead, or nobody.

- With several components on one new issue, the first by name that names
  someone decides the assignee. It applies only when the issue is created
  without an assignee.
- Deleting a component that issues use asks whether they just lose it or move
  to another component.

**Versions.** Versions mark the releases of the product. An issue says which
version fixes it and which versions it affects. The list shows each version's
dates, its state and how many issues are still unresolved. Its order is the
order of the version lists elsewhere in Solstice: change it with the arrows.
Names are unique in the project, and the start date cannot be after the
release date.

- **Release** sets the release date (today by default). When issues fixed in
  the version are not resolved, you can move them to another version that is
  neither released nor archived, or leave them where they are. "Unrelease"
  reverts it.
- **Archive** keeps the version but marks it archived; you can restore it. An
  archived version is not offered as a place for unresolved issues on release.
- **Merge** moves every issue that uses the version, as fixed or affected
  version, to another version, then deletes it.
- **Delete** asks which version the issues fixed in it, and those it affects,
  get in its place, or none.
- The moves are recorded in the issues' history.

**Permissions and roles.** A table of what each holder may do. The
permissions are: browse, create, edit, comment, change status, assign,
assignable, attach files, link, log work, delete, moderate, manage watchers,
security level and administer. A permission is granted to everyone, a user, a
group, a role, the project lead, the reporter or the assignee.

Watch for:

- "Administer" lets someone configure the project. Global administrators
  configure every project, but they do not see issue content unless a
  permission lets them.
- "Moderate" allows editing or removing other people's comments, work logs and
  files. Grant it sparingly.
- A permission with no holder is granted to nobody. After a Jira import, read
  the import report: permissions Solstice could not translate are removed,
  never widened.

**Workflows.** See below.

**Notifications.** Whether the project sends e-mail about its issues, and who
else receives it.

- "Send email notifications for this project" turns off all e-mail about the
  project's issues when unchecked, whatever each person chose in their own
  settings. It needs working e-mail (see E-mail).
- "More recipients" is a table of events against the instance's groups and the
  project's roles. A group or role checked for an event also receives that
  event's e-mail in this project, on top of the people who already follow the
  issue. Everyone still chooses, in their own settings, which events reach them.

### Workflows

A workflow sets the statuses of an issue and the moves allowed between them.
Workflows belong to a project. Each issue type follows one of the project's
workflows. The default workflow takes every type that follows no other.

- A workflow has an initial status (where new issues start), a list of
  statuses and a table of transitions. Tick a box to allow a move from the
  row's status to the column's. A row "Any status" allows a move from anywhere.
  The transition's name is the button shown on the issue.
- You can create a workflow from a copy of an existing one.
- Statuses are common to all projects, and so are their names. Adding a status
  that does not exist creates it, with a category.
- When you remove a status that issues are in, or delete a workflow that
  leaves issues in statuses the default workflow lacks, a dialog asks where
  those issues go. Each move is recorded in the issue's history.
- Deleting a workflow sends its issue types back to the default workflow.
  Issues keep their current status.
- When an issue reaches a status in the "Done" category without a resolution,
  it gets the default resolution if one is set.
- Jira conditions, validators and post-functions do not exist in Solstice.
  Permissions decide who can make a transition.
- Without the "administer" permission, workflows are read only.

## Issues

These catalogues are shared by all projects. Each list is in display order:
drag the rows or use the arrows. Clear the search to reorder.

### Issue types

Name, description and whether the type is a sub-task type. Watch for:

- The sub-task box can only change while no issue uses the type.
- Deleting a type used by issues, or offered by a project, asks you to move
  its issues to another type of the same kind (sub-task or not). The last type
  cannot be deleted.

### Priorities

Name, description and a color (`#rrggbb`). One priority is the default, given
to issues created without one. Without a default, the middle priority is used.
Deleting a used priority asks where to move its issues.

### Resolutions

How an issue was closed. The default resolution is given to issues that reach
a done status without one. Without a default, they stay unresolved. Deleting
a used resolution asks where to move its issues.

### Statuses

Each status has a category: to do, in progress or done. Boards and reports use
the category, whatever the status is called. A status in use cannot be deleted.
"Where used" lists the workflows, board columns and issues that hold it; take
it out of those first. Deleted statuses keep their name in issue history.

### Link types

A name and two verbs, for example "blocks" and "is blocked by". Deleting a
used link type asks which type its links move to.

### Custom fields

Fields added to issues, on top of the built-in ones. Types: one-line text,
multi-line text, link, choice (one or several values), radio buttons,
checkboxes, number, date, date and time, labels, one user, several users.

- A field applies to all projects and issue types, or only to the ones you
  choose. It appears on an issue when both its project and its type match.
- The type of a field cannot change after creation.
- Options can be added, renamed, reordered and disabled. A disabled option
  keeps its existing values but can no longer be chosen. Prefer disabling to
  deleting.
- Deleting an option that issues use asks you to move their value to another
  option or to clear it.
- Deleting a field that holds values asks you to type its name. The values go
  with it and cannot be recovered. Boards that estimate with the field fall
  back to no estimate, and saved filters that name it stop matching.
- Fields imported from Jira keep their Jira type. A type Solstice cannot edit
  is shown as "Jira type, display only". A cascading field imported from Jira
  shows only its first level of options.

## E-mail and integrations

### E-mail

Solstice sends notifications, daily digests, filter subscriptions, reminders,
invitations and password reset links through one SMTP server.

- Set the server, the port and the connection security: STARTTLS (port 587,
  the usual choice), TLS from the start (port 465) or none (port 25, for a
  relay on an internal network). Add a user name and password if the server
  requires them, and a sending address.
- The password is stored encrypted with the secret key. Back that key up with
  the database (see [backups.md](backups.md)).
- If you change the server, port, security or user name, type the password
  again or remove it.
- "Send a test email" uses the saved settings and shows the server's exact
  reply when it fails. Run it after every change.
- The outbox shows pending, failed and recently sent messages. "Retry failed
  sends" is useful after fixing the settings.
- Unchecking "Send email" keeps the settings but stops all sending.

### Webhooks

A webhook sends a POST in Jira's webhook format to another tool when
something changes.

- Choose the address, the events (issues, comments, time logs, sprints,
  boards) and, optionally, a JQL query: only issues that match trigger a
  delivery.
- With a secret, each delivery carries `X-Hub-Signature: sha256=…`, an
  HMAC-SHA256 of the body. The recipient should check it. The secret is stored
  encrypted.
- "Send an empty body" is for a service that only checks the signature.
- "Send a test call" posts to the address at once and shows the response.
- A delivery that fails is tried up to 6 times, after 30 seconds, 2 minutes,
  10 minutes, 30 minutes and 2 hours. Each webhook keeps its last 100
  deliveries, with the response of each.
- Webhooks failing in the last 24 hours show on the system page.

Watch for:

- A webhook sees every issue, confidential ones included. Restrict it with its
  JQL query.
- Targets may be on your private network or local machine. Link-local
  addresses and cloud metadata addresses are always refused, redirects are not
  followed and no proxy is used.

### Access tokens

Personal access tokens let scripts, CI jobs and IDE plugins sign in as a
person without their password. People create their own under their settings
(see [rest-api.md](rest-api.md)). This page lists the tokens of every account,
with their last use and expiry, and revokes any of them at once.

Policy, "Who may create tokens":

- **Administrators only** (the default).
- **Members of a group.**
- **Everyone.**

Administrators always may. Tightening the policy does not revoke existing
tokens: revoke them from the list. A token cannot create other tokens.
Resetting a password through a reset link also deletes the account's tokens.

The same policy says who may authorize applications (MCP clients) through
OAuth (see [mcp.md](mcp.md)). Next to it:

- **Applications stay signed in for (days)**: an authorized application
  unused this long must be authorized again. 30 days by default.
- **Let applications register themselves** (on by default): MCP clients
  without a client metadata document register through Dynamic Client
  Registration. Turned off, new registrations are refused; applications
  already registered keep working.

"Authorized applications", below the tokens, lists what everyone authorized:
the account, the application, its scopes, when it was authorized and last
used. A **self-registered** application chose its own name, which nothing
verifies. Revoking an authorization ends the application's tokens at once;
it must ask its user again to come back.

## System

### Audit log

A read-only record of who signed in and who changed accounts, access rights,
projects, workflows, catalogues, settings, webhooks, tokens, imports and
updates, with the address and time. Filter by kind of action, user, dates or a
word in the target or details. Open an entry to see the details, such as the
old and new value of a setting.

Retention: entries older than the retention period are deleted automatically,
once an hour. The default is 365 days. Choose 30 days or more, or 0 to keep
everything. Shortening the period deletes the older entries at once, after a
confirmation, and this cannot be undone. Keep a database backup first if you
need the older entries.

### Jira import

Replaces everything Solstice holds with the data of a Jira database. This is
the most destructive page of the administration. Read
[migration.md](migration.md) before using it. It asks for a backup
confirmation and for the database name to be typed. Solstice is in maintenance
while it runs.

### System information

Version, Go version, start time, the database server, name and size, the data
directory and the space it uses, the configuration file and listen address,
counts of accounts, projects, issues, comments and attachments, the e-mail
queue, failing webhooks, applied database migrations and a running import. No
secret is shown. Look here first when something seems wrong: a growing
"failed" count in the mail queue, or a failing webhook, shows up here.

### Updates

Shows the installed version and the latest release published on GitHub.

The Updates page itself is not reachable yet: its address (`/admin/updates`)
has no route in the interface until issue #28 is done. Until then, the
"Updates" block of the System page shows whether a newer version is available,
whether Solstice is up to date or has not checked yet, when it last checked,
and why a check failed. It does not offer the actions below.

What the Updates page does:

- Solstice checks for new versions at most every 12 hours, and sends nothing
  but the request for the latest version number. You can turn the automatic
  checks off, or use "Check now". This is the only request Solstice makes on
  its own.
- On a Linux installation made with the installer, "Update" downloads the
  release, verifies it, backs up the database, restarts, and puts the previous
  version back if the new one does not start. You confirm with your password.
  Solstice is unavailable for a few seconds; signed-in people stay signed in.
- In a container or after a manual installation, the page shows the commands
  to run instead.

See [upgrades.md](upgrades.md) before updating a production instance.
