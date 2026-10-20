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
- 429: too many sign-in attempts. The `Retry-After` header and the `retryAfter`
  field of the body both give the wait in whole seconds. 413: a JSON body over
  the size limit of the route; the message gives the limit in KiB. Any other
  failure is a 500 with a generic message, the cause is only in the server log.
- Creations answer 201: besides those listed below, `POST
  /api/issues/{key}/links`, `POST /api/issues/{key}/attachments` and `POST
  /api/admin/fields/{id}/options`.
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
Changing the password or the e-mail address needs `currentPassword` (400
"Mot de passe actuel incorrect.", throttled like sign-ins); `newPassword`
must be at least `passwordMinLength` characters and changing the password
signs the user's other sessions out. An address change is refused to a
personal access token (403); an unchanged address needs nothing. Answers `GET /api/me`'s object. (`profile.update`,
`password.change`)

### `PUT /api/me/avatar`, `DELETE /api/me/avatar`

`PUT` takes a multipart body (as attachments do) whose first file is the
picture: a PNG or JPEG of 1 MiB at most and 512 × 512 pixels at most (400
when it is neither or does not decode, 413 when too heavy). The server stores
a re-encoded PNG, so no metadata or extra bytes survive; the interface sends a
256 × 256 square. The image must be upright: the EXIF orientation is not
applied (it is dropped with the other metadata), so a client sends the
picture already turned the right way. Both answer 204; `DELETE` also answers
204 when there is no picture. `GET /api/avatars/{userId}` shows it (section 3).

---

## 3. Users — global admin

- `GET /api/admin/users?q=` → `[adminUser]` (200 max), where
  `adminUser` = `{id, username, name, email, active, admin, hasPassword,
  created, lastLogin (instant | null), groups: [idName]}`.
- `GET /api/admin/users/{id}` → `adminUser` plus `tokens` (number of
  personal access tokens).
- `GET /api/people/{username}` → `{user: userRef, email, assigned: [...]}`.
  `email` is empty unless the caller is an administrator or the person.
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
  As in the profile, the password and a new address are refused for the
  administrator's own account (400: the profile changes them) and through a
  personal access token (403).
- `PUT /api/admin/users/{id}/groups` `{groups: [groupId]}` → `adminUser`.
  Replaces the memberships. (`group.members`)
- `PUT /api/admin/users/{id}/avatar` (multipart, as `PUT /api/me/avatar`) and
  `DELETE /api/admin/users/{id}/avatar` → 204: set or remove an account's
  picture. (`user.update`, with the detail `avatar: set | remove`)
- `GET /api/avatars/{userId}` (every signed-in user) → the picture as
  `image/png` (`X-Content-Type-Options: nosniff`), or the initials as an SVG
  when the account has none; 404 for an unknown account. Both come with
  `Cache-Control: private, max-age=60` and an `ETag`, and answer 304 to
  `If-None-Match`, so a change shows within a minute. Jira's
  `GET /secure/useravatar?ownerId={username}` answers the same.
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
  `simple` (To do / In progress / Done). Statuses are reused by name or
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
With `&project=WEB` (a key of an existing project, current or former; 404
otherwise) the former keys of that project are not counted as taken, as when
changing its key.

### Read

`GET /api/projects` lists active projects. `?archived=1` also lists archived
ones. Each project now has `archived` (bool).

`GET /api/projects/{key}` (unchanged shape) now also has
`project.archived`, `project.categoryId` (number | null) and, in
`issueTypes`, every type allowed in the project.

`GET /api/admin/projects` — global admin. Every project, seen or not:
`[{id, key, name, kind, lead (userRef), category (idName | null), archived,
issues}]`, active first, then archived, by name.

### Update — project admin

`PATCH /api/projects/{key}`
`{name?, description? (null clears), lead?, category? (id | null),
defaultAssignee? ("unassigned" | "lead"), issueTypes?: [typeId]}`
→ the settings object: `{id, key, name, description, kind, lead (userRef),
category (idName | null), defaultAssignee, archived, issueTypes: [typeRef],
issues}`. Removing an issue type still used by issues: 409 `{inUse:
[{type: typeRef, issues}]}`. (`project.update`)

### Project settings — project admin

`GET /api/projects/{key}/settings` → `{id, key, name, description, kind,
lead (userRef), category (idName | null), defaultAssignee, archived,
issueTypes: [typeRef], issues}` — the project as its administrators
configure it: the fields `PATCH /api/projects/{key}` changes, with `id`,
`key`, `kind`, `archived` and `issues` (the issue count).

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

### Change the key — global admin

`POST /api/projects/{key}/rename` (it was `POST
/api/admin/projects/{key}/rename` on the development branch only, never in a
release) with body `{"newKey": "SHOP",
"confirm": "WEB"}` → the `GET /api/projects/{key}` settings object, with the
new key. `confirm` must be the current key (409 otherwise). The new key
follows the rules of a new project (`GET /api/admin/project-key?key=`):
400 if ill-formed, 409 if used by a project now or formerly. The exception
is a key this project had itself: it may take it back (that key then stops
being a former key; `project-key?project=` reports it as free). Issue keys are
not stored (they are the project key and the issue number), so every issue
takes the new key at once; the old key is kept as a former key, and `OLD-12`
still finds its issue in `/api/issues/OLD-12`, `/rest/api/2/issue/OLD-12`,
`/api/projects/OLD` and in JQL (`project = OLD`, `key = OLD-12`), in the
simple search (`project:OLD`, `OLD-12`), in the `project` parameter of
`/api/issues`, `/api/epics`, `/api/versions` and `/api/components`, and as
the `project` of a new issue. Saved filters, board filters and other texts
that name the old key are left as they are. Changes of keys (this one and
the creation of a project) wait for each other; if another one has not ended
after 10 seconds, the answer is 503 and the call can be repeated.
(`project.rename`, details `{oldKey, newKey}`)

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

- `GET /api/admin/fields/types` → the types a field can have:
  `[{type, typeKey, options}]`, each with its Jira key (`typeKey` =
  `com.atlassian.jira.plugin.system.customfieldtypes:` + kind) and `options`
  true for the option types.
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
  `POST /api/admin/fields/{id}/options` `{value}` → 201 field detail,
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
`GET /api/settings/public`, and `GET /api/admin/import` and `GET /api/me` for
the administrator who started it (their session is recognised without the
database). Poll `GET /api/admin/import` every second or two; when `state` is
`done` or `failed`, the application is back.

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
  tokens. Administrators always may; the default is administrators only.
  Creating a token against the policy answers 403. Tokens that exist keep
  working.
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

---

## 12. Notification preferences

`GET|PUT /api/me/notifications` — everyone signed in:

- GET → `{delivery, digestHour, ownChanges, events, dueReminder,
  reminderHour, timeZone, mailEnabled}`.
  - `delivery`: `immediate` (within a couple of minutes, default) | `digest`
    (once a day) | `none` (no notification e-mails).
  - `digestHour`, `reminderHour`: 0–23, default 8, in `timeZone` (the
    server's time zone).
  - `ownChanges`: also e-mail the user's own changes (default false).
  - `dueReminder`: a daily e-mail of issues due (default false).
  - `events`: one switch per group — `created`, `assigned`, `commented`,
    `mentioned`, `status`, `updated`, `attachments`, `deleted` (all true by
    default).
  - `timeZone` and `mailEnabled` are read-only (the hours' time zone above;
    whether this Solstice sends e-mail at all).
- PUT any subset `{delivery?, digestHour?, ownChanges?, events? (one switch
  per group), dueReminder?, reminderHour?}`; fields left out keep their
  value. 400 for an unknown delivery or event group, or an hour outside
  0–23. Answers GET. E-mails already waiting follow: `none` drops them, a
  new digest hour moves the digest.

`GET|PUT /api/projects/{key}/notifications` — project admin:

- GET → `{enabled, extra, events}`; `events` lists the groups a project may
  send to extra people: `created`, `assigned`, `commented`, `status`,
  `updated`, `attachments`, `deleted`.
  - `enabled` (default true): false stops every notification e-mail about
    the project's issues.
  - `extra`: `[{event, kind: group|role, id, name}]` — a group or role that
    also receives one of those events.
- PUT `{enabled?, extra?}`; `extra` replaces the list, each entry
  `{event, kind: "group"|"role", id}`. 400 for an unknown event group or a
  group or role that does not exist. Answers GET.

---

## 13. E-mail, webhooks and updates — global admin

### Mail

- `GET /api/admin/mail` → `{enabled, host, port, security, username, from,
  fromName, passwordSet, baseUrl}` — the SMTP settings. The password is
  never sent back, only `passwordSet` (bool); `baseUrl` (string | null) is
  the public address links in e-mails are built from. `security`: `none` |
  `starttls` (default) | `tls`; an empty saved configuration means STARTTLS
  and its usual port.
- `PUT /api/admin/mail` any subset of `{enabled, host, port, security,
  username, password, from, fromName}` → the settings as GET. The password
  is left out to keep it, `null` or `""` to remove it; keeping it while
  changing the server, port, security or user name is refused (400):
  whoever can change the destination must send the password again rather
  than collect it on a server of their own. `security: none` with a user
  name is refused (400) unless the host is `localhost` or a loopback
  address; credentials are only ever sent over TLS or STARTTLS, or to the
  local machine. The test e-mail and the queue show the error
  `credentials are not sent over an unencrypted connection` for a saved
  configuration that breaks this rule. Validation (400): unknown security,
  port outside 1–65535, carriage return or line feed in the host, user name
  or display name, `enabled` without a host, an invalid sender address.
  (`settings.mail`, details: changed fields, the password only as changed
  or removed)
- `POST /api/admin/mail/test` `{to?}` → `{sent: true, to}`: sends a test
  e-mail now with the saved settings (`to` empty: the administrator's own
  address). 400 for an invalid address; 409 when no host or sender is
  saved; 502 with the SMTP server's own words (connect, TLS, auth,
  sender, recipient…).
- `GET /api/admin/mail/queue` → `{pending, failed, sentLastDay,
  oldestPending, recent, errors, batching, digests}`: messages waiting,
  given up on, sent in the last day, the oldest waiting (instant | null),
  the last messages and the last failures (newest first) each `[{id, kind,
  to, subject, status, attempts, lastError (string | null), created,
  nextAttempt (instant | null), sent (instant | null)}]`, and the
  notification batches waiting now (`batching`) and the digest batches
  (`digests`).
- `POST /api/admin/mail/queue/retry` → `{retried}`: failed messages go back
  in the queue (resets and invitations excepted).

### Webhooks

A webhook POSTs the event to its address. `delivery` = `{id, event, status
(pending|sent|failed), statusCode, durationMs, attempts, error, created,
attempted, nextAttempt}`; `statusCode`, `durationMs`, `error`, `attempted`
and `nextAttempt` are null when unknown.

- `GET /api/admin/webhooks` → `[webhook]`, `webhook` = `{id, name, url,
  events, jql, hasSecret, enabled, excludeBody, created, updated,
  lastDelivery (delivery | null)}`.
- `GET /api/admin/webhooks/events` → the events Solstice sends: `[event]` —
  `jira:issue_created`, `jira:issue_updated`, `jira:issue_deleted`,
  `comment_created`, `comment_updated`, `comment_deleted`,
  `worklog_created`, `worklog_updated`, `worklog_deleted`,
  `sprint_created`, `sprint_started`, `sprint_updated`, `sprint_closed`,
  `sprint_deleted`, `board_created`, `board_updated`, `board_deleted`.
- `GET /api/admin/webhooks/{id}` → webhook; 404 when unknown.
- `POST /api/admin/webhooks` `{name, url, events, jql?, secret?, enabled?,
  excludeBody?}` → 201 webhook. (`webhook.create`) `PUT
  /api/admin/webhooks/{id}` the same fields, all optional → webhook.
  (`webhook.update`)
  - `name` required, 255 characters max; `url` (2000 max) must be
    `http(s)://` with a host, and must not resolve to a link-local or cloud
    metadata address (400).
  - `events`: at least one of the names above (400).
  - `jql`: only issues matching it are sent; empty means every issue. The
    query is checked when saved (400).
  - `secret` (255 max) is sealed and never sent back (`hasSecret`); left
    out it is kept, `""` removes it.
  - `enabled` defaults to true on creation; `excludeBody` sends the event
    without the issue's payload.
- `DELETE /api/admin/webhooks/{id}` → 204. (`webhook.delete`) The audit log
  never holds the secret nor the address's user name, path and query.
- `GET /api/admin/webhooks/{id}/deliveries` → the last 100 deliveries
  `[delivery]`, newest first.
- `POST /api/admin/webhooks/{id}/test` → the delivery of a `solstice:test`
  event sent now.

### Updates

Solstice learns of the releases published on GitHub; that check is the only
request it makes on its own, at most every 12 hours, and never once checks
are turned off.

- `GET /api/admin/updates` → `{current, latest, newer, notes, publishedAt,
  releaseUrl, checkedAt, checkError, checksEnabled, install, canApply,
  commands, download, lastAttempt}`.
  - `latest` (string | null) and `newer`: a release is published, and is it
    newer than `current`; `notes` (Markdown), `publishedAt` (instant |
    null), `releaseUrl`.
  - `checkedAt` (instant | null) and `checkError` (`unreachable` |
    `refused` | `invalid` | null): the last check.
  - `checksEnabled` (default true); `install`: `systemd` | `docker` |
    `manual`.
  - When `newer`: `canApply` (true on a systemd installation with no update
    waiting or running), and for docker and manual the `commands` to run
    and, for manual, the `download` archive for this machine.
  - `lastAttempt` (instant | null): `{at, from, to, status, log}` with
    `status` `pending` | `running` | `ok` | `rolled_back` | `failed` — the
    update helper's work (Linux installations only).
- `POST /api/admin/updates/check` → the same, checking again now (at most
  once every 30 seconds; asking sooner answers the previous check).
  (`update.check`)
- `PUT /api/admin/updates/settings` `{checksEnabled}` → the state. 400 when
  the field is missing. (`update.settings`)
- `POST /api/admin/updates/apply` `{version, password}` → 202 the state:
  the helper is asked to install `version`. The administrator's password is
  asked again; 403 when wrong. (`update.denied`) 409: an installation that
  cannot update from the interface, `version` not the latest release,
  nothing newer to install, or another update waiting or running.
  (`update.request`) The outcome reaches the audit log once the application
  is back (`update.result`).

---

## 14. Issue security levels — global admin

Each project has its own levels (the import copies a Jira security scheme
into each project). An issue at a level is seen only by the level's members;
a level with no member hides its issues from everyone. Every route below
is for global administrators only (403 otherwise, whatever the project): a
project administrator could add themselves to a confidential level, or delete
it and expose its issues. 404 for an unknown project, and for a level of
another project.

- `GET /api/projects/{key}/security-levels` → `[{id, name, description,
  issues, memberCount, members: [{id, holder, user: userRef|null, group:
  idName|null, role: idName|null}]}]`, by name. `issues` is how many issues
  are at the level.
- `POST /api/projects/{key}/security-levels` `{name, description?}` → 201
  level. Names unique in the project (409).
- `PATCH /api/projects/{key}/security-levels/{id}` `{name?, description?
  (or null)}` → level.
- `DELETE /api/projects/{key}/security-levels/{id}` body optional `{moveTo?:
  levelId, clear?: bool}` → 204. Issues still at the level (sub-tasks
  included) make it 409 `{error, usage: {issues}}`, unless `moveTo` names
  another level of the project (400 otherwise) or `clear` is true: those
  issues then have no level, so everyone who sees the project sees them.
  Both together is 400. The change is written to each issue's history.
- `POST /api/projects/{key}/security-levels/{id}/members` `{holder, id?}` →
  201 level. `holder` is one of exactly the kinds the permission checks
  know: `everyone` (any signed-in account), `user`, `group`, `role` (these
  three with the `id` of the user, group or project role), `lead` (the
  project lead), `reporter`, `assignee` (of each issue). Already a member:
  409; unknown holder or id: 400.
- `DELETE /api/projects/{key}/security-levels/{id}/members/{member}` → 204;
  `{member}` is a `members[].id`.

Changes to levels and members apply at once (the permission cache is
refreshed). They are written to the audit log on the project (target type
`project`, label the key) as `securitylevel.create`, `securitylevel.update`,
`securitylevel.delete`, `securitylevel.member_add` and
`securitylevel.member_remove` (family `securitylevel`), with details
`{securityLevel, …}`: `name` and `description` as `{from, to}` for an update,
`issues`, `members`, `movedTo`, `cleared` for a delete, `holder` and `member`
for the members.

---

## 15. Moving an issue — signed in

`POST /api/issues/{key}/move` moves an issue, and its sub-tasks, to another
project and answers `{key}`, the new key. The old key keeps resolving, as do
the keys the source project went by before (`moved_issue_keys`). The issue
gets the next number of the target, and its history gets lines for what
changed (`Key`, `project`, and the rest below). It is announced to webhooks as
`issue_moved`, and written to the audit log as `issue.move`.

Solstice has no separate move permission: the right to move is **Edit** in the
issue's current project (and on each of its sub-tasks), plus **Create** in the
target project, which the user must also be able to browse. A sub-task cannot
be moved alone.

Request, with `project` (a key) the only field always needed:

```json
{"project": "KEY", "check": false, "type": "bug", "subtaskType": "sub-task",
 "status": "open", "components": [3], "affectsVersions": [4], "fixVersions": [5],
 "securityLevel": 2, "restricted": {"to": "role", "role": 7}}
```

- `check: true` is a dry run: nothing is written, the answer says what is
  still to choose and what the move changes.
- `type`, `subtaskType` and `status` are ids; the others are ids of the
  target project's own components, versions and security levels. A list given
  is the whole list the issue ends with; `securityLevel` is an id, or `null`
  for none.
- `restricted` is `{"to": "role", "role": id}`, `{"to": "group", "group":
  id}` or `{"to": "refuse"}`.

What is matched by itself: the type, when the target has it; the status, when
it is in the target's workflow for that type; components and versions by name
(case aside); the security level by name; the assignee, when they may be
assigned in the target, else the rule of a new issue there (the components'
default assignee, then the project's). A sub-task whose type the target lacks
takes `subtaskType`; one whose status is not in the target's workflow starts
at its initial status. Values of custom fields that do not apply in the
target are dropped, and an open sprint on a board whose filter names projects
but not the target is left. `status` may only be given when the issue's own is
not in the target's workflow (it needs the Transition right otherwise: 400).

The dry run answers 200 with:

```json
{"project": {"id": 2, "key": "KEY", "name": "Name"}, "ready": false,
 "missing": ["type", "components", "restricted"],
 "types": [], "subtaskTypes": [], "statuses": [],
 "components": {"unmatched": [], "options": []},
 "affectsVersions": {"unmatched": [], "options": []},
 "fixVersions": {"unmatched": [], "options": []},
 "securityLevel": {"unmatched": [], "options": []},
 "restricted": {"comments": 0, "worklogs": 0, "roles": [], "groups": []},
 "droppedFields": [], "sprints": [],
 "assignee": {"from": null, "to": null},
 "subtasks": {"count": 0, "reset": 0, "lost": 0, "reassigned": 0}}
```

- `missing` names the choices still to make, `ready` is true when it is empty.
  `types`, `subtaskTypes`, `statuses` and each `unmatched`/`options` pair are
  only there when a choice is asked for: `unmatched` is what the issue has
  that the target has no same-named match for, `options` what the target
  offers. Lists are never `null`.
- `securityLevel`: the target's levels are listed only to a user with the
  Security right in both projects.
- `restricted`: the comments and work logs restricted to a project role the
  user can read, since roles are held per project; `roles` are the target's
  roles that have a member (empty to a user who cannot browse the whole
  target), `groups` the groups the user belongs to (all of them for an
  administrator). The ones the user cannot read are not offered a choice: they
  take the role of the same name in the target (one with a member there). An
  item already restricted to a group keeps it. A global administrator reads
  them all.
- `droppedFields` (names), `sprints` (names) and `assignee` (`userRef`s,
  present when it changes) say what is lost or changed. `subtasks.reset`
  counts the sub-tasks that start over at an initial status, `lost` the
  components and versions they leave behind, `reassigned` the ones whose
  assignee cannot be assigned in the target.

Without `check`, the same plan is made and then applied in one transaction;
a move with choices still missing is a 400 `{"error", "missing": [...]}`.

Errors:

- 404: the issue, or the target project, is not visible to the user.
- 403: no Edit right on the issue or a sub-task, or no Create right in the
  target; moving an issue that has a security level without the Security
  right in the target, or choosing a level without it in both projects.
- 400: no `project`, or the issue's own project; a sub-task; a choice that is
  not one of the target's (type, status, component, version, level, role,
  group); `restricted` set to `refuse`; comments restricted to a role the user
  cannot read, with no same-named role in the target (the message says an
  administrator can move the issue); sub-tasks and no sub-task type in the
  target; choices still missing.
- 409: the issue or a sub-task changed after the plan was made, or while the
  move ran. Nothing moved; reload and try again.
