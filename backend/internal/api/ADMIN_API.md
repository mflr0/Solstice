# Administration API

The endpoints that make Solstice configurable from its own interface. The
configuration file only bootstraps the server (database URL, data directory,
listen address); everything below is stored in the database and applies at
once, without a restart.

Conventions (as for the rest of `/api`):

- JSON in and out, camelCase. Request bodies reject unknown fields (400).
- Every request other than GET/HEAD carries the header `X-Solstice: 1` (403
  otherwise).
- Errors: `{"error": "message"}` in the request's language
  (`Accept-Language`, else the user's language, else the instance's default
  language). Some errors carry more fields, listed below.
- 401: not signed in. 403: signed in but not allowed. 404: unknown, or a
  project the user cannot see (keys cannot be probed). 400: invalid input,
  with a message saying what to fix. 409: conflicts (in use, already exists…).
- "Global admin" means `admin: true` in `GET /api/me`. "Project admin" means
  the `admin` permission in that project (`can.configure` in
  `GET /api/projects/{key}`); global admins count as project admins for
  configuration (not for content).
- Dates: `YYYY-MM-DD` for days, RFC 3339 for instants (UTC).
- `userRef` = `{id, username, name, active}`; `idName` = `{id, name}`;
  `typeRef` = `{id, name, subtask}`.
- Every change below is written to the audit log (`GET /api/admin/audit`),
  action names in parentheses.

Status: every endpoint below is live.

---

## 1. General settings

### `GET /api/settings/public` — no sign-in needed

```json
{
  "instanceName": "Solstice",
  "defaultLanguage": "fr",
  "banner": {"text": "Maintenance ce soir à 20 h", "level": "warning"},
  "passwordMinLength": 12,
  "emailLinks": true
}
```

`banner` is `null` when disabled or empty. `level`: `info` | `warning`.
`emailLinks` is true when e-mail is on and complete and the public address is
set: only then can reset links and invitations be sent (the sign-in page shows
"forgot your password" accordingly, the user dialog the invitation option).

### `GET /api/admin/settings` — global admin

```json
{
  "instanceName": "Solstice",
  "baseURL": "https://tickets.example.org",
  "timeZone": "Europe/Paris",
  "defaultLanguage": "fr",
  "banner": {"enabled": false, "text": "", "level": "info"},
  "sessionDays": 30,
  "passwordMinLength": 12,
  "sources": {
    "instanceName": "default", "baseURL": "env", "timeZone": "file",
    "defaultLanguage": "database", "banner": "default",
    "sessionDays": "default", "passwordMinLength": "default"
  },
  "languages": ["fr", "en", "de", "it", "hi"]
}
```

- `baseURL`: public address, `""` when unknown (links in e-mails are then
  left out). An `https://` address makes session cookies `Secure`.
- `timeZone`: IANA name; `""` = the server's local zone. Used for dates typed
  in searches (JQL), e-mail hours, digests, CSV exports.
- `defaultLanguage`: for people who chose none, and for e-mails to them.
- `sessionDays`: how long a sign-in lasts (1–365). Applies to new sign-ins.
- `passwordMinLength`: 8–128, default 12. Applies to new passwords (admin,
  profile, reset); existing passwords keep working.
- `sources.<field>`: where the value in effect comes from — `database` (set
  here), `file` (configuration file or environment variable), `default`.
  Only `baseURL` and `timeZone` can come from the file or the environment
  (`SOLSTICE_HTTP_BASEURL`, `SOLSTICE_TZ`): those are initial
  values; once saved here, the database wins.

### `PUT /api/admin/settings` — global admin

Body: any subset of the fields above. A field set to `null` is removed from
the database and goes back to its inherited value (file/env/default).
`sources` and `languages` may be sent back and are ignored. Answers like GET.
(`settings.general`, details: changed fields with from/to)

Validation (400): empty or too long instance name (100 max); base URL not
`http(s)://host[:port]` without path; unknown time zone; unsupported language;
banner text over 500 characters or unknown level; numbers out of range.

---

## 2. Profile (everyone signed in)

### `PATCH /api/me`

```json
{"name": "Marie Durand", "email": "marie@example.org", "locale": "fr",
 "currentPassword": "…", "newPassword": "…"}
```

All fields optional. `locale`: a language or `null` (follow the browser).
Changing the password needs `currentPassword` (400 "Mot de passe actuel
incorrect.", throttled like sign-ins) and `newPassword` of at least
`passwordMinLength` characters; other sessions of the user are signed out.
Answers `GET /api/me`'s object. (`profile.update`, `password.change`)

---

## 3. Users — global admin

- `GET /api/admin/users?q=` → `[adminUser]` (200 max), where
  `adminUser` = `{id, username, name, email, active, admin, hasPassword,
  created, lastLogin (instant | null), groups: [idName]}`.
- `GET /api/admin/users/{id}` → `adminUser` plus `tokens` (number of
  personal access tokens).
- `POST /api/admin/users` `{username, name, email, password, admin,
  groups?: [groupId], invite?: bool}` → 201 `{id}`. (`user.create`) With
  `invite` the account has no password (`password` must be empty, `email` is
  required) and its person gets an e-mail with a link, valid 7 days, to choose
  one: 409 when `emailLinks` is false. (`user.invite`)
- `POST /api/admin/users/{id}/invite` → `{sent: true}`: sends the invitation
  again (the previous link stops working); only for an active account with an
  address that never signed in. (`user.invite`)
- `PATCH /api/admin/users/{id}` `{name?, email?, active?, admin?,
  password?}` → 204. (`user.update`, `user.deactivate`, `admin.grant`, …)
- `PUT /api/admin/users/{id}/groups` `{groups: [groupId]}` → `adminUser`.
  Replaces the memberships. (`group.members`)
- `POST /api/admin/users/{id}/reset-password` → `{password}`: a generated
  password, shown once; the user's sessions end. (`password.reset`) With the
  body `{"method": "link"}` it sends the account a link, valid one hour, to
  choose its own password and answers `{sent: true}`; nothing changes until
  the link is used. (`password.reset_request`)

### Forgotten password — no sign-in needed

- `POST /api/auth/forgot` `{identifier}` (user name or e-mail address) → always
  `200 {ok: true}`, whether or not an account matches and whether or not
  e-mail is set up, and never faster than 400 ms. A matching active account
  with an address gets an e-mail (to that address, with a link built from the
  public address); asking again within a minute, or too often, mails nothing.
  429 when one client address asks too often. (`password.reset_request`)
- `GET /api/auth/reset/{token}` → `{purpose: reset | invite, name, username}`,
  or 400 when the link is unknown, used, expired, replaced or its account is
  deactivated.
- `POST /api/auth/reset` `{token, password}` → the signed-in `me`: sets the
  password (the usual minimum length), ends every session and personal access
  token of the account, kills its other links and signs the person in.
  (`password.reset_done`, `user.invite_accept`)

---

## 4. Groups — global admin

- `GET /api/admin/groups?q=` → `[{id, name, description, members}]`
  (`members`: count).
- `GET /api/admin/groups/{id}` → `{id, name, description, members:
  [userRef], usage: {grants, roles, securityLevels, comments, worklogs}}`.
- `POST /api/admin/groups` `{name, description?}` → 201 group (as in the
  list). Names are stored lower-case, unique (409). (`group.create`)
- `PATCH /api/admin/groups/{id}` `{name?, description?}` → group. Renaming
  keeps comments and work logs restricted to the group restricted to it.
  (`group.update`)
- `DELETE /api/admin/groups/{id}` → 204. 409 `{usage}` while comments or
  work logs are restricted to the group (deleting it would hide them from
  everyone). Grants, role memberships and security level memberships go with
  the group. (`group.delete`)
- `POST /api/admin/groups/{id}/members` `{users: [userId]}` → group detail.
- `DELETE /api/admin/groups/{id}/members/{userId}` → 204. (`group.members`)

## 5. Project roles

Roles are shared by all projects; who holds them is set per project.

- `GET /api/admin/roles` → `[{id, name, description, projects, grants}]`
  (projects with members in the role; grants naming the role). Global admin.
- `POST /api/admin/roles` `{name, description?}` → 201 role. (`role.create`)
- `PATCH /api/admin/roles/{id}` `{name?, description?}` → role.
  (`role.update`)
- `DELETE /api/admin/roles/{id}` → 204; 409 `{usage: {comments,
  worklogs}}` while comments or work logs are restricted to it.
  (`role.delete`)

Per project — project admin:

- `GET /api/projects/{key}/roles` → `[{id, name, description, users:
  [userRef], groups: [idName]}]` (every role, with this project's members).
- `PUT /api/projects/{key}/roles/{roleId}` `{users: [userId], groups:
  [groupId]}` → that role's entry. Replaces its members in this project.
  (`project.roles`)

Grants themselves stay on `GET|PUT /api/projects/{key}/permissions`
(holders: `everyone`, `user`, `group`, `role`, `lead`, `reporter`,
`assignee`).

---

## 6. Projects

### Create — global admin

`POST /api/projects`

```json
{
  "key": "WEB", "name": "Site web", "description": "…",
  "lead": 12, "category": 3,
  "template": "software",
  "access": {"holder": "group", "id": 4},
  "issueTypes": ["10001", "10002"],
  "defaultAssignee": "unassigned"
}
```

- `key`: upper-case letters and digits, starts with a letter, 2–10
  characters, unique among current **and former** keys (409).
- `name` required, unique (409); `lead` an active user (default: the
  creator); `category` a category id or null.
- `template`: `software` (statuses To Do / In Progress / In Review / Done) or
  `simple` (À faire / En cours / Terminé). Statuses are reused by name or
  created; the project gets one workflow where any status can be reached
  from any other.
- `access`: who else works in the project — `{"holder": "group", "id"}`,
  `{"holder": "role", "id"}` (the lead is added to the role in this
  project), `{"holder": "everyone"}`, or `null`/absent (only the lead).
  That holder gets browse, create, edit, comment, transition, assign,
  assignable, attach, link, worklog. The lead gets every permission; global
  admins configure every project anyway.
- `issueTypes`: default all issue types.
- On the very first project of a fresh instance, empty catalogues (issue
  types, priorities, resolutions, link types, roles) are filled with
  defaults in the instance's language.

→ 201 `{id, key}`. 400 messages name the field at fault. (`project.create`)

`GET /api/admin/project-key?key=WEB` → `{key, available: true}` or
`{key, available: false, reason}` — for checking as one types. Global admin.

### Read

`GET /api/projects` lists active projects. `?archived=1` also lists archived
ones. Each project now has `archived` (bool).

`GET /api/projects/{key}` (unchanged shape) now also has
`project.archived`, `project.categoryId` (number | null) and, in
`issueTypes`, every type allowed in the project.

### Update — project admin

`PATCH /api/projects/{key}`
`{name?, description? (null clears), lead?, category? (id | null),
defaultAssignee? ("unassigned" | "lead"), issueTypes?: [typeId]}`
→ the `GET /api/projects/{key}` object. Removing an issue type still used
by issues: 409 `{inUse: [{type: typeRef, issues}]}`. (`project.update`)

### Archive — global admin

`POST /api/projects/{key}/archive`, `POST /api/projects/{key}/unarchive` →
the `GET /api/projects/{key}` object. An archived project disappears from
project lists and pickers (versions, components, issue creation); its issues
stay readable by those who could see them, but nothing in it can be changed.
(`project.archive`, `project.unarchive`)

### Delete — global admin

`DELETE /api/projects/{key}` with body `{"confirm": "WEB"}`. An empty project
(no issues) needs no confirmation. Otherwise 409 `{issues: n}` unless
`confirm` is the project key. Everything in the project goes: issues and all
they hold (comments, attachments and their files, history…), components,
versions, workflows, grants, role members, security levels, former keys.
Boards and filters remain (they may show nothing). (`project.delete`)

### Categories

- `GET /api/project-categories` (signed in) → `[{id, name, description,
  projects}]`.
- `POST /api/admin/project-categories` `{name, description?}`,
  `PATCH /api/admin/project-categories/{id}`,
  `DELETE /api/admin/project-categories/{id}` (projects lose the category).
  Global admin. (`category.create|update|delete`)

### Components — project admin to change, browse to read

- `GET /api/projects/{key}/components` → `[{id, name, description, lead:
  userRef|null, assigneeType, archived, issues}]`.
- `POST /api/projects/{key}/components` `{name, description?, lead?,
  assigneeType?}` → 201 component. Names unique in the project (409).
- `PATCH /api/projects/{key}/components/{id}` `{name?, description?,
  lead? (id | null), assigneeType?, archived?}` → component.
- `DELETE /api/projects/{key}/components/{id}` body optional
  `{moveTo: componentId}` → 204. Issues lose the component, or get `moveTo`.
- `assigneeType`: `project_default` (default), `component_lead`,
  `project_lead`, `unassigned`. A new issue created without assignee takes
  the default of its first component (by name) that names someone.
  (`component.create|update|delete`)

### Versions — project admin to change, browse to read

- `GET /api/projects/{key}/versions` → `[{id, name, description,
  startDate, releaseDate, released, archived, position, issues: {total,
  unresolved}}]`, in order.
- `POST /api/projects/{key}/versions` `{name, description?, startDate?,
  releaseDate?}` → 201 version (last in order). Names unique in the project.
- `PATCH /api/projects/{key}/versions/{id}` `{name?, description?,
  startDate?, releaseDate?, released?, archived?}` → version. Dates may be
  `null`; the start cannot be after the release date.
- `POST /api/projects/{key}/versions/order` `{ids: [versionId]}` → the list.
- `POST /api/projects/{key}/versions/{id}/release`
  `{releaseDate?: "YYYY-MM-DD" (default today), moveUnresolvedTo?: versionId}`
  → version. Unresolved issues (no resolution) fixed in this version move to
  `moveUnresolvedTo` (an unreleased version of the project), with history;
  without it they stay.
- `POST /api/projects/{key}/versions/{id}/merge` `{into: versionId}` → 204.
  Issues (affects and fix) move to `into`, with history; the version is
  deleted.
- `DELETE /api/projects/{key}/versions/{id}` body optional
  `{fixTo?: versionId, affectsTo?: versionId}` → 204. Issues lose the
  version, or get the replacement.
  (`version.create|update|release|merge|delete`)

---

## 7. Catalogues — global admin

Shared by all projects. Each list is in display order.

| Catalogue | List | Item fields | Usage counted |
|---|---|---|---|
| Issue types | `GET /api/admin/issue-types` | `id, name, description, subtask, position` | `issues, projects` |
| Priorities | `GET /api/admin/priorities` | `id, name, description, color, position, default` | `issues` |
| Resolutions | `GET /api/admin/resolutions` | `id, name, description, position, default` | `issues` |
| Link types | `GET /api/admin/link-types` | `id, name, inward, outward, position` | `links` |
| Statuses | `GET /api/admin/statuses` | `id, name, category, position` | `usage: {issues, workflows, boardColumns}` |

For `{kind}` in `issue-types`, `priorities`, `resolutions`, `link-types`:

- `POST /api/admin/{kind}` → 201 item. Names unique (409).
  - issue types: `{name, description?, subtask?}`
  - priorities: `{name, description?, color?: "#rrggbb"}`
  - resolutions: `{name, description?}`
  - link types: `{name, inward, outward}`
- `PATCH /api/admin/{kind}/{id}` same fields, all optional → item.
  Priorities also take `default: true` (the priority of new issues created
  without one; otherwise the middle one), and so do resolutions (the
  resolution of issues reaching a done status without one; otherwise they
  stay unresolved). One default per catalogue: setting it moves it.
  Deleting the default leaves none. An issue type's `subtask` flag
  only changes while no issue uses it (409).
- `POST /api/admin/{kind}/order` `{ids: [...]}` → the list.
  `POST /api/admin/statuses/order` too.
- `DELETE /api/admin/{kind}/{id}` body optional `{moveTo: id}` → 204.
  Unused: deleted. Used (for an issue type, by an issue or by a project that
  offers it): 409 `{usage: {...}}` unless `moveTo` names another
  item of the same catalogue (for issue types, of the same sub-task kind);
  its issues (or links) then move there, with history. The last issue type
  cannot go.
- Statuses: create/rename/delete stay on `POST /api/statuses`,
  `PATCH|DELETE /api/statuses/{id}`. `GET /api/admin/statuses/{id}/usage` →
  where one is used: `{workflows: [{id, name, project: {id, key, name}}],
  boards: [{id, name, column}], projects: [{key, name, issues}]}`.

(`issuetype.*`, `priority.*`, `resolution.*`, `linktype.*`, `status.order`)

---

## 8. Custom fields — global admin

Types: `text`, `textarea`, `number`, `date`, `datetime`, `select`,
`multiselect`, `checkboxes`, `radio`, `user`, `multiuser`, `labels`, `url`.
They are stored with Jira's type keys, so fields imported from Jira and
fields created here behave alike (issue view, REST API, JQL).

- `GET /api/admin/fields` → `[field]`, `field` = `{id, name, description,
  type (one of the above, or null for a Jira type Solstice only displays),
  typeKey, options (count, option types only), issues (issues with a value),
  context: {projects: [{id, key, name}], issueTypes: [typeRef]}}`.
  Empty `projects` / `issueTypes` mean all.
- `GET /api/admin/fields/{id}` → `field` with `options: [{id, value,
  disabled, position}]` instead of a count (`[]` for other types).
- `POST /api/admin/fields` `{name, description?, type, options?: [string],
  context?: {projects: [projectKey], issueTypes: [typeId]}}` → 201 field
  detail. Names unique among custom fields (409).
- `PATCH /api/admin/fields/{id}` `{name?, description?, context?}` → detail.
  The type never changes.
- `DELETE /api/admin/fields/{id}` body optional `{confirm: "<field name>"}`
  → 204. A field holding values needs the confirmation: 409 `{usage:
  {issues, boards}}` otherwise. Values go; boards estimating with it fall
  back to none. Saved filters naming it stop matching.
- Options (`select`, `multiselect`, `checkboxes`, `radio`):
  `POST /api/admin/fields/{id}/options` `{value}`,
  `PATCH /api/admin/fields/{id}/options/{optionId}` `{value?, disabled?}`,
  `POST /api/admin/fields/{id}/options/order` `{ids}` → field detail.
  Each option of the detail counts its `issues` (those with it, or an option
  under it, chosen) and its `children` (options under it, for a cascading field
  imported from Jira; only the top level is listed).
  A disabled option keeps its values but cannot be chosen any more.
  `DELETE /api/admin/fields/{id}/options/{optionId}` body optional
  `{moveTo?: optionId, clear?: true}` → field detail. Options under it (a
  cascading field's children) go too. An option issues have chosen: 409
  `{usage: {issues}}` unless `moveTo` names another option of the field at the
  same level (400 otherwise; their value moves there, an issue already holding
  it keeps one) or `clear` is true (their value goes).
  (`field.create|update|delete|options`)

For issue forms (signed in, browse permission):
`GET /api/projects/{key}/fields?issueType={id}` → `[{id, name, type,
typeKey, options: [{id, value}]}]` — the fields offered for that project and
type (enabled options only).

Issue values: `POST /api/issues` and `PATCH /api/issues/{key}` accept
`"customFields": {"<fieldId>": value}`, value as in Jira's REST API: string
(text, textarea, url, date `YYYY-MM-DD`, datetime RFC 3339), number,
`{"id": "<optionId>"}` or `"<option value>"`, a list of those for multi
fields, a user name (or `{"name": …}`) for user fields, a list of strings for
labels, `null` to clear. A field outside its context for the issue's project
and type is refused (400). JQL finds new fields by name or
`cf[<id>]`, as imported ones. The REST API (`/rest/api/2/issue`) applies the
same checks.

Moving an issue: `POST /api/issues/{key}/move` (signed in) → `{key}`, the new
key; the old key keeps resolving (`moved_issue_keys`). Solstice has no
separate move permission: the right to move is **Edit** in the issue's
current project (and on each of its sub-tasks), plus **Create** in the target
project, which the user must also be able to browse. A security level is
matched by name (case aside) with a level of the target project; when there is
none, the dry run lists the target's levels only to a user with the Security
right there, and otherwise reports just that the current level has no match.
Comments and work logs restricted to a project role (on the issue and its
sub-tasks) cannot keep it, since roles are held per project: the dry run counts
them (`restricted: {comments, worklogs, roles, groups}`, with the target's
roles that have a member, and the groups the user belongs to, all of them for an
administrator) and the move needs `"restricted": {"to": "role", "role": id}`,
`{"to": "group", "group": id}` or `{"to": "refuse"}` (400, nothing moves);
without it the move is a 400 `missing: ["restricted"]`. Items restricted to a
group only stay as they are; the choice is made in the move's transaction.
Written to the audit log as `issue.move`.

---

## 9. System — global admin

`GET /api/admin/system`

```json
{
  "version": "1.0.0", "goVersion": "go1.25.1", "startedAt": "…", "uptimeSeconds": 3600,
  "database": {"server": "MariaDB 11.4.2", "name": "solstice", "sizeBytes": 123456789},
  "dataDir": {"path": "/var/lib/solstice", "sizeBytes": 987654321, "error": null},
  "counts": {"usersActive": 120, "usersInactive": 30, "projects": 12, "projectsArchived": 2,
             "issues": 45000, "comments": 120000, "attachments": 8000, "attachmentBytes": 4567890},
  "mail": {"enabled": true, "pending": 3, "failed": 1, "sentLastDay": 250, "oldestPending": "…" },
  "webhooks": {"total": 4, "failing": [{"id": 2, "name": "CI", "failed": 5, "lastError": "…", "lastAttempt": "…"}]},
  "migrations": {"count": 10, "latest": "0010_admin", "list": [{"version": "0001_init", "appliedAt": "…"}]},
  "config": {
    "path": "/etc/solstice/solstice.conf", "found": true, "addr": ":8080"
  },
  "import": null
}
```

`failing` lists webhooks whose deliveries failed in the last 24 hours.
`dataDir.sizeBytes` is recomputed at most every 5 minutes. No secret is ever
shown (database password, secret key, SMTP password). `import`: the running
or last import from the administration (see 10), or null.

---

## 10. Import from Jira — global admin

- `GET /api/admin/import` → `{confirm: "<database name>", job: importView |
  null}`. `confirm` is what the administrator must type to start.
- `POST /api/admin/import/test` `{url, jiraHome, tz}` → `{engine, projects,
  issues, users, attachments, warning?}` (same checks as the setup wizard).
- `POST /api/admin/import` `{url, jiraHome, tz, confirm}` → 202
  `importView`. 400 when `confirm` is not the database name; 409 while
  another import runs (here, in the setup wizard, or `solstice import` on the
  same database). (`import.start`, then `import.done` / `import.failed`)

`importView` = `{state: running|done|failed, phase, done, total, rows,
steps: [{name, imported, skipped}], warnings, error?, seconds}` (as in the
setup wizard).

**While the import runs, Solstice is in maintenance:** every other `/api` and
`/rest` request answers 503 `{"error": …, "maintenance": true}`, except
`GET /api/settings/public` and `GET /api/admin/import` for the administrator
who started it (their session is recognised without the database). Poll
`GET /api/admin/import` every second or two; when `state` is `done` or
`failed`, the application is back.

### What a replacing import keeps

An import always replaces Solstice's data with Jira's (there is no merge).
Solstice's own configuration survives it, from the administration and from
`solstice import -replace` alike:

- **Kept as is:** instance settings (general, e-mail including the SMTP
  password, audit retention…), webhooks (their delivery log is emptied),
  the audit log.
- **Kept for the accounts that still exist after the import** (matched by
  user name): personal access tokens, notification preferences, interface
  language, last sign-in, open sessions, and Solstice administrator rights
  (someone made administrator in Solstice stays one).
- **Accounts created in Solstice** (not from Jira) whose user name does not
  exist in Jira are kept, with their password, but without groups or roles
  (those come from Jira).
- **The administrator who starts the import** from the administration keeps
  their account, password, administrator rights and session, even when Jira
  has an account of the same name (Solstice's password wins for that one
  account).
- **Replaced:** users from Jira, groups, projects, issues and everything in
  them, catalogues, custom fields, filters, boards, sprints. Gone with them:
  filter subscriptions, project notification settings, e-mails waiting to be
  sent, notification batches, webhook deliveries. Audit entries stay; their
  link to an account is restored by user name.

---

## 11. Access tokens and audit log

Personal access tokens (`Authorization: Bearer sol_…`):

- `GET /api/me/tokens` → the user's own tokens `[{id, name, created, expires,
  lastUsed, expired}]`; `POST /api/me/tokens` `{name, expires?}` → 201 with the
  same plus `token`, the secret, readable this once; `DELETE
  /api/me/tokens/{id}`. (`token.create`, `token.revoke`)
- `GET /api/me/tokens/policy` → `{canCreate}`: whether the current user may
  create a token now (false for a request signed in with a token, and for
  users the policy below leaves out).
- `GET /api/admin/tokens?user=<name>` → everyone's tokens, each with its
  `user` (`userRef`); `DELETE /api/admin/tokens/{id}` revokes any.
- `GET /api/admin/tokens/settings` → `{who: everyone|admins|group, group:
  idName | null}`; `PUT` the same with `group` as an id: who may create
  tokens. Administrators always may; the default is everyone. Creating a
  token against the policy answers 403. Tokens that exist keep working.
  (`settings.tokens`)

Audit log:

- `GET /api/admin/audit?user=&action=&from=&to=&q=&page=` → `{total, page, size,
  entries: [{id, created, user, username, ip, action, target, details}]}`,
  newest first, 50 a page. `action` is exact or a family (`user` matches
  `user.*`); `from`/`to` are dates (inclusive) or instants; `q` searches the
  target, user name, address and details.
- `GET /api/admin/audit/settings` → `{retentionDays}`; `PUT` the same:
  `0` keeps everything, otherwise 30 to 36500 days (400 below 30).
  Shortening purges at once. (`settings.audit`)

Filter subscriptions (scheduled e-mail of a filter's results):

- `GET /api/filters/{id}/subscriptions`, `POST` (create), `PUT
  /api/filters/{id}/subscriptions/{sid}`, `DELETE …/{sid}`, `POST …/{sid}/run`
  work on one filter. Body: `{schedule: daily|weekly, days: [1-7], hour:
  0-23, emptyToo, group?}`; `group` (an id) subscribes a group, administrators only.
- `GET /api/me/subscriptions` → the subscriptions the user receives (their own
  or a group's they belong to) or created, on the filters they still see,
  each as above plus `filterName`, by filter name. Administrators get their
  own here, not everyone's.
