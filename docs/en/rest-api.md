# REST API

Solstice answers a subset of the Jira Server and Data Center REST APIs, so
that scripts, CI jobs and IDE plugins written for Jira keep working:

- `/rest/api/2` and its alias `/rest/api/latest`
- `/rest/agile/1.0` and its alias `/rest/agile/latest`

Version 3 of the Jira API is not offered. The server reports itself as Jira
9.12.0 on `serverInfo`, with `serverTitle` set to `Solstice`, so that clients
that test the version accept it.

Examples use the host `solstice.example.org` and the token `sol_example`.
Real tokens are longer.

The interface has its own API under `/api`. It is not Jira-compatible and is
not described here.

## Authentication

Every route except `GET /rest/api/2/serverInfo` needs a signed-in user.
Without credentials the answer is 401.

| Method | How |
|---|---|
| Personal access token | `Authorization: Bearer sol_example` |
| Basic, with a token | `Authorization: Basic …` where the user name is the account's and the password is the token |
| Basic, with a password | The account's user name and password. Failed attempts are throttled like the sign-in form, and answer 429 when the limit is hit. |
| Session cookie | The cookie the interface sets at sign-in. |

If a request has an `Authorization` header, it is used, and the cookie is
ignored. Prefer tokens to passwords: a token can be revoked on its own, and
guessing someone's password cannot lock their scripts out.

Create a token under "My settings", "Access tokens". The secret is shown once.
Only people allowed by the administrator's policy can create tokens (see
[administration.md](administration.md)), and a request signed in with a token
cannot create another one.

```sh
curl -H "Authorization: Bearer sol_example" \
  https://solstice.example.org/rest/api/2/myself
```

A request sees and does what its user may see and do in the interface: the
same permissions, restricted comments, security levels and history apply.
Something the user cannot see answers 404; something visible but not permitted
answers 403.

### Writes: the forgery header rule

A request that changes something (anything other than GET, HEAD or OPTIONS)
must be provably not forged by another site. It is accepted when one of these
holds:

- It carries the header `X-Atlassian-Token: no-check`, or the header
  `X-Solstice` with any value.
- It is signed in with a Bearer token. Browsers never add one by themselves.
- It is signed in with Basic credentials, and it is not a POST with a body
  type an HTML form can send: no `Content-Type`,
  `application/x-www-form-urlencoded`, `multipart/form-data` or `text/plain`.

A request signed in with the session cookie must always carry one of the two
headers. Otherwise the answer is 403.

In practice, a script with a token needs no extra header. A script with Basic
credentials needs `X-Atlassian-Token: no-check` to upload an attachment, which
is a multipart POST:

```sh
curl -u alice:sol_example \
  -H "X-Atlassian-Token: no-check" \
  -F "file=@report.pdf" \
  https://solstice.example.org/rest/api/2/issue/DEMO-1/attachments
```

## Conventions

- Bodies are JSON. Unknown fields in a request body are ignored, because Jira
  clients send many that Solstice has no use for.
- Errors use Jira's format:
  `{"errorMessages": ["…"], "errors": {"field": "…"}}`. Messages are in the
  request's language (`Accept-Language`), else the user's, else the
  instance's default.
- Dates are written like Jira: `2026-10-15T09:12:00.000+0200`, in the
  instance's time zone. Requests also accept RFC 3339.
- Paging uses `startAt` and `maxResults`. The defaults and maximums are:
  50 and 1000 for search, user lists and the issues of a board, a backlog, a
  sprint or an epic; 50 and 50 for boards and sprints; 5000 for comments and
  work logs. `maxResults=0` only counts.
- Issues can be named by key (`DEMO-1`) or by numeric id. A former project key
  still resolves.
- A request to an address under `/rest/` that is not listed below answers 404.

## Issues

| Request | Notes |
|---|---|
| `POST /rest/api/2/issue` | Create an issue. 201 with `id`, `key` and `self`. |
| `GET /rest/api/2/issue/{key}` | Parameters `fields` and `expand`. By default all fields. |
| `PUT /rest/api/2/issue/{key}` | Edit with `fields` and `update`. 204. |
| `DELETE /rest/api/2/issue/{key}` | Refused when the issue has sub-tasks, unless `?deleteSubtasks=true`. 204. |
| `PUT /rest/api/2/issue/{key}/assignee` | `{"name": "alice"}`, `{"name": null}` for nobody, `{"name": "-1"}` for the project's default assignee. 204. |
| `GET /rest/api/2/issue/{key}/watchers` | The count to everyone, the names to people who manage watchers. |
| `POST /rest/api/2/issue/{key}/watchers` | The body is a user name as a JSON string; empty means yourself. |
| `DELETE /rest/api/2/issue/{key}/watchers` | `?username=`; empty means yourself. |
| `POST /rest/api/2/issue/{key}/votes` | Vote. 204. |
| `DELETE /rest/api/2/issue/{key}/votes` | Remove the vote. 204. |
| `POST /rest/api/2/issueLink` | Link two issues. `inwardIssue` is the source: `{"type": {"name": "Blocks"}, "inwardIssue": {"key": "A-1"}, "outwardIssue": {"key": "A-2"}}` means "A-1 blocks A-2". An optional `comment` is added to the source issue. 201. |
| `DELETE /rest/api/2/issueLink/{id}` | Remove a link. |
| `GET /rest/api/2/issueLinkType` | The link types. |

`expand` on an issue accepts `changelog`, `renderedFields`, `names` and
`transitions` (with `transitions.fields`).

Fields Solstice can set on create and edit: `project`, `issuetype`,
`summary`, `description`, `environment`, `duedate`, `timetracking`
(`originalEstimate`, `remainingEstimate`), `labels`, `priority`,
`resolution`, `assignee`, `security`, `components`, `fixVersions`, `versions`,
`parent` (sub-tasks, on create), custom fields as `customfield_<id>`, and the
Jira Software sprint and epic link fields. In `update`, the operations are
`set`, `edit`, `add` and `remove` (`add` and `remove` for labels, components
and versions), and `{"comment": [{"add": {"body": "…"}}]}` adds a comment.

Limits:

- An issue cannot be moved to another project or to another parent through
  the API.
- `reporter` can only name the user making the request (on create) or the
  current reporter. Solstice records who really created an issue.
- A field Solstice does not know answers 400 with the field name.

Custom field values follow Jira's shapes: strings, numbers, `{"id": "…"}` or
`{"value": "…"}` for options, lists for multiple choices, user names for user
fields. A field outside its project and issue type context is refused.

## Search and JQL

| Request | Notes |
|---|---|
| `GET /rest/api/2/search` | Parameters `jql`, `startAt`, `maxResults`, `fields`, `expand`. |
| `POST /rest/api/2/search` | The same in a JSON body. |

`fields` defaults to `*navigable`. The answer has `startAt`, `maxResults`,
`total` and `issues`, and `names` when asked with `expand=names`.

Solstice implements the Jira Query Language itself. What works:

- **Boolean logic.** `AND`, `OR`, `NOT`, parentheses.
- **Operators.** `=`, `!=`, `>`, `>=`, `<`, `<=`, `~`, `!~`, `IN`, `NOT IN`,
  `IS EMPTY`, `IS NOT EMPTY` (also `NULL`).
- **Fields.** `project`, `status`, `statusCategory`, `type` (`issuetype`),
  `priority`, `resolution`, `assignee`, `reporter`, `creator`, `labels`,
  `component`, `fixVersion`, `affectedVersion`, `sprint`, `"Epic Link"`,
  `parent`, `level`, `watcher`, `commenter`, `voter`, `created`, `updated`,
  `resolved`, `due`, `key` (`issue`, `id`), `summary`, `description`,
  `environment`, `text`, `comment`, `timespent`, `originalEstimate`,
  `remainingEstimate`, and custom fields by name or as `cf[10001]`.
  `participant`, `participants` and `involves` are a Solstice extension: any
  role a person has on an issue.
- **Functions.** `currentUser()`, `membersOf("group")`, `now()`,
  `startOfDay()`, `endOfDay()`, `startOfWeek()`, `endOfWeek()`,
  `startOfMonth()`, `endOfMonth()`, `startOfYear()`, `endOfYear()` (each with
  an optional offset such as `"-1d"`), `openSprints()`, `closedSprints()`,
  `futureSprints()`, `releasedVersions()`, `unreleasedVersions()`,
  `earliestUnreleasedVersion()`, `latestReleasedVersion()` (each with
  optional project arguments), `standardIssueTypes()`, `subtaskIssueTypes()`.
- **Dates.** `2026-10-15`, `"2026-10-15 14:30"`, the same with `/`, and
  relative values such as `-7d`, `2w`, `-1M` (units `y`, `M`, `w`, `d`, `h`,
  `m`). They are read in the instance's time zone. Weeks start on Monday.
- **Durations.** `2h`, `1d 4h`, `1w`. A day is 8 hours and a week 5 days.
- **History.** `WAS`, `WAS NOT`, `WAS IN`, `WAS NOT IN` and `CHANGED`, with
  `AFTER`, `BEFORE`, `ON`, `DURING`, `BY`, `FROM` and `TO`.
- **Sorting.** `ORDER BY` on `key`, `created`, `updated`, `resolved`, `due`,
  `priority`, `status`, `summary`, `assignee`, `reporter`, `project`, `type`,
  `rank`, `fixVersion`, `affectedVersion`, `component`, `resolution`, the
  estimates, `timespent`, `"Epic Link"`, `sprint` and custom fields. Without
  `ORDER BY`, the most recently updated issues come first.
- `resolution = Unresolved` finds issues without a resolution.

Limits:

- History clauses work on `status`, `priority`, `resolution`, `assignee`,
  `reporter` and `fixVersion` only.
- `rank`, `"Epic Name"`, `"Epic Status"` and `"Epic Color"` cannot be used as
  search clauses. `rank` can be used in `ORDER BY`.
- A function that is not listed above is refused with an error message, and so
  is an unknown field.
- `issueHistory()` is accepted and matches nothing: Solstice does not record
  which issues people viewed.
- `commenter` ignores restricted comments.
- Invalid JQL answers 400 with the position and the reason. There is no
  `validateQuery` option and no autocomplete endpoint.

## Transitions

| Request | Notes |
|---|---|
| `GET /rest/api/2/issue/{key}/transitions` | The moves the user may make now. `?transitionId=` keeps one. `expand=transitions.fields` adds fields. |
| `POST /rest/api/2/issue/{key}/transitions` | `{"transition": {"id": "21"}}`, or by `name`. `fields` and `update` apply with the move. 204. |

A transition that is not available answers 400 on `transition`.

## Comments

| Request | Notes |
|---|---|
| `GET /rest/api/2/issue/{key}/comment` | `startAt`, `maxResults`, `orderBy`. Only comments the user may read. |
| `POST /rest/api/2/issue/{key}/comment` | `{"body": "…", "visibility": {"type": "group", "value": "staff"}}`. `type` is `group` or `role`. 201. |
| `GET /rest/api/2/issue/{key}/comment/{id}` | |
| `PUT /rest/api/2/issue/{key}/comment/{id}` | |
| `DELETE /rest/api/2/issue/{key}/comment/{id}` | |

## Work logs

| Request | Notes |
|---|---|
| `GET /rest/api/2/issue/{key}/worklog` | |
| `POST /rest/api/2/issue/{key}/worklog` | `timeSpent` (`"1h 30m"`) or `timeSpentSeconds`, `started`, `comment`. 201. |
| `GET /rest/api/2/issue/{key}/worklog/{id}` | |
| `PUT /rest/api/2/issue/{key}/worklog/{id}` | |
| `DELETE /rest/api/2/issue/{key}/worklog/{id}` | |

Limits: the `adjustEstimate` options are not supported; the remaining estimate
always adjusts by itself. A `visibility` on a work log is refused.

## Attachments

| Request | Notes |
|---|---|
| `POST /rest/api/2/issue/{key}/attachments` | Multipart upload; the field name does not matter. Answers the list of attachments added. Needs `X-Atlassian-Token: no-check` with Basic or cookie sign-in. |
| `GET /rest/api/2/attachment/{id}` | The attachment's metadata. |
| `DELETE /rest/api/2/attachment/{id}` | |
| `GET /secure/attachment/{id}/{filename}` | The file itself, at the address Jira uses. |

The size of one upload is capped by `SOLSTICE_HTTP_MAXUPLOAD` (100 MiB by
default). An attachment imported without its file answers 404 on download.

## Projects, versions and components

| Request | Notes |
|---|---|
| `GET /rest/api/2/project` | The projects the user can see. |
| `GET /rest/api/2/project/{key}` | By key, former key or id. Includes issue types, components and versions. |
| `GET /rest/api/2/project/{key}/statuses` | The statuses of each issue type's workflow. |
| `GET /rest/api/2/project/{key}/versions` | |
| `GET /rest/api/2/project/{key}/components` | |

Projects, versions and components are read only through the REST API.

## Users

| Request | Notes |
|---|---|
| `GET /rest/api/2/myself` | The user of the request. |
| `GET /rest/api/2/user?username=` | Or `?key=`. Groups are listed only for the user themself and for administrators. |
| `GET /rest/api/2/user/search?username=` | Matches user name, display name and e-mail address. `.` matches everyone. `includeActive` and `includeInactive` choose which accounts. |
| `GET /rest/api/2/user/assignable/search?project=` | Or `?issueKey=`. Active users who can be assigned issues. |
| `GET /rest/api/2/mypermissions` | With `projectKey`, `projectId`, `issueKey` or `issueId`; otherwise in at least one project. |
| `GET /secure/useravatar`, `GET /secure/projectavatar` | Avatars are generated: initials on a color. |

Users cannot be created, changed or deleted through the API, and there are no
group endpoints. `mypermissions` maps Jira's permission names to Solstice's,
which are fewer and broader.

## Metadata

| Request | Notes |
|---|---|
| `GET /rest/api/2/serverInfo` | No sign-in needed. |
| `GET /rest/api/2/field` | System and custom fields. |
| `GET /rest/api/2/priority` | |
| `GET /rest/api/2/status` | |
| `GET /rest/api/2/issuetype` | |
| `GET /rest/api/2/resolution` | |

There is no `createmeta` or `editmeta` endpoint.

## Filters

| Request | Notes |
|---|---|
| `GET /rest/api/2/filter/{id}` | A saved filter the user may see. |
| `GET /rest/api/2/filter/favourite` | The user's favourite filters. |

Filters are read only through the REST API. Run a filter by searching with its
`jql`.

## Agile: boards and sprints

Under `/rest/agile/1.0`:

| Request | Notes |
|---|---|
| `GET /board` | Parameter `projectKeyOrId`. |
| `GET /board/{id}` | |
| `GET /board/{id}/configuration` | Columns, filter and estimation. |
| `GET /board/{id}/sprint` | Scrum boards only. `state=active,future,closed`. |
| `GET /board/{id}/issue` | The board's issues, in its columns. |
| `GET /board/{id}/backlog` | |
| `GET /sprint/{id}` | |
| `POST /sprint/{id}`, `PUT /sprint/{id}` | Change name, goal, dates, and state. Both work as a partial update: what is not given is left alone. `state: "active"` starts a future sprint; `state: "closed"` completes an active one, and its open issues return to the backlog. |
| `DELETE /sprint/{id}` | Only a sprint that has not started; its issues return to the backlog. |
| `GET /sprint/{id}/issue` | |
| `POST /sprint/{id}/issue` | `{"issues": ["DEMO-1"]}`. Into an open sprint. |
| `POST /backlog/issue` | `{"issues": ["DEMO-1"]}`. Out of their open sprint. |
| `PUT /issue/rank`, `POST /issue/rank` | `{"issues": ["DEMO-1"], "rankBeforeIssue": "DEMO-2"}`, or `rankAfterIssue`. Exactly one of the two. |
| `GET /epic/{key}` | |
| `GET /epic/{key}/issue` | |

Boards cannot be created or changed, and sprints cannot be created, through
the API.

## Not supported

Anything not listed above is not registered and answers 404. Notably:

- Version 3 of the Jira API, and Jira Cloud's APIs.
- Creating or changing users, groups, projects, versions, components,
  filters, boards, and custom fields.
- Moving an issue to another project, bulk operations, `createmeta` and
  `editmeta`, the changelog as its own endpoint, remote links, issue
  properties, dashboards, and Jira Service Management APIs.
- Plugin APIs.
