# MCP endpoint

Solstice has a built-in [Model Context Protocol](https://modelcontextprotocol.io)
server at `/mcp`, so that AI agents (Claude Code, IDE assistants) can search,
read and change issues. It speaks the Streamable HTTP transport, without sessions:
every request carries its own credentials and gets a JSON answer.

Examples use the host `solstice.example.org` and the token `sol_example`.

## Signing in

The endpoint accepts a bearer token, in the header
`Authorization: Bearer …`: an OAuth access token that the client gets by
having you sign in (below), or a personal access token. Passwords, Basic
credentials, session cookies and tokens in the address are refused.

### With OAuth

Clients that support MCP authorization (claude.ai connectors, Claude Code,
some IDEs) only need the address `https://solstice.example.org/mcp`. On the
first request they open the browser on Solstice: you sign in as usual
(password, LDAP or single sign-on), and a consent page names the
application, the address it goes back to, and the scopes it asks for. Untick
what it does not need, then allow or deny. `read` is always given; `admin`
never is, and no tool needs it.

Solstice is the OAuth 2.1 authorization server of `/mcp`, as the
[MCP authorization spec](https://modelcontextprotocol.io/specification/latest/basic/authorization)
describes:

- its metadata (RFC 8414) is at
  `https://solstice.example.org/.well-known/oauth-authorization-server`;
- the authorization code flow, with PKCE (`S256` only), the `iss` parameter
  in the answer (RFC 9207), and the `resource` parameter required and equal to
  `https://solstice.example.org/mcp` (RFC 8707);
- clients are identified by a Client ID Metadata Document: their `client_id`
  is an HTTPS address serving a JSON document with `client_id`, `client_name`
  and `redirect_uris`. Solstice fetches it when you authorize, without
  following redirects and never from a private or loopback address;
- clients without such a document register through Dynamic Client
  Registration (RFC 7591), at `https://solstice.example.org/register`, which
  the metadata names as `registration_endpoint`. Only public clients
  (`token_endpoint_auth_method` `none`), with `client_name` and redirect
  addresses in HTTPS or on a loopback address (`http://127.0.0.1:…`,
  `http://localhost:…`, any port). An address may register 10 clients an
  hour. A client nobody authorizes within a day is deleted. Its name is its
  own claim: the consent page says so instead of naming a host. An
  administrator can turn registration off; clients already registered keep
  working;
- access tokens (`solo_…`) last one hour. Refresh tokens (`solor_…`) are
  replaced at each use; one used twice revokes every token it led to. A
  refresh token unused for 30 days expires (an administrator can change the
  number of days next to the token policy), and the client asks you again;
- these tokens work on `/mcp` only: the interface's API and the REST API
  refuse them.

Who may authorize an application follows the token policy, as for personal
access tokens (see [administration.md](administration.md)); an application
already authorized keeps working if the policy changes. Registrations,
authorizations, refusals and revocations go in the audit log, and
administrators can list and revoke every authorization.

"My settings", "Connected apps" lists the applications you authorized, with
their scopes and last use. Revoking one ends its tokens at once. Changing
your password, or the deactivation of your account, revokes them all.

### With a personal access token

The header is `Authorization: Bearer sol_example`. Create a token under "My settings", "Access tokens", and give it the `read`
scope, plus `issues:write` if the agent is to change issues, `comments:write`
to comment, `worklogs:write` to log work and `agile:write` to plan sprints. Only people allowed by the administrator's token policy can create
tokens (see [administration.md](administration.md)). A token keeps working
if the policy changes later, as on the REST API.

A request without a valid token answers 401 with
`WWW-Authenticate: Bearer resource_metadata="https://solstice.example.org/.well-known/oauth-protected-resource"`.
That document (RFC 9728) names the resource, its authorization server and
the scopes tokens can have.

## What the agent can do

An agent sees and does what the token's owner may see and do in the
interface: the same permissions, workflows, security levels and restricted
comments apply, and the history records the owner as the author of changes. A secured
issue the owner cannot see stays hidden, from search and by key.

The tools answer in the JSON of the Jira Server REST API (see
[rest-api.md](rest-api.md)). Each one needs a scope; the tool list only shows
the tools the token's scopes allow, and calling another one fails with the
scope it needs. Every tool of this table needs `read`.

| Tool | Gives |
|---|---|
| `search_issues` | Issues matching a JQL query, a page at a time |
| `get_issue` | One issue, its fields, comments, work log, links, attachments |
| `pick_issues` | Issues whose key or summary matches a text |
| `get_transitions` | Transitions the user can take on an issue |
| `list_comments`, `get_comment` | Comments of an issue |
| `list_worklogs`, `get_worklog` | Work logged on an issue |
| `get_watchers` | Who watches an issue |
| `list_remote_links`, `get_remote_link` | Links from an issue to outside pages |
| `get_attachment` | An attachment's metadata and download address |
| `get_create_meta`, `get_edit_meta` | Fields that can be set on new or existing issues |
| `get_my_permissions` | The user's permissions, globally, in a project or on an issue |
| `list_projects`, `get_project`, `get_project_statuses` | Projects and their statuses |
| `get_myself`, `get_user`, `search_users`, `search_assignable_users` | Users |
| `search_groups`, `list_group_members` | Groups and their members |
| `list_fields`, `list_issue_types`, `list_statuses`, `list_priorities`, `list_resolutions`, `list_issue_link_types` | The instance's catalogues |
| `get_server_info` | The server's version and address |
| `list_favourite_filters`, `get_filter` | Saved filters: the user's favourites, or one by id |

These tools need `issues:write`:

| Tool | Does |
|---|---|
| `create_issue`, `edit_issue`, `delete_issue` | Creates, changes or deletes an issue |
| `assign_issue` | Assigns an issue, or unassigns it |
| `transition_issue` | Moves an issue along its workflow |
| `add_watcher`, `remove_watcher` | Starts or stops someone watching an issue |
| `add_vote`, `remove_vote` | Votes for an issue, or takes the vote back |
| `add_attachment`, `delete_attachment` | Attaches a file (base64-encoded, up to the upload limit), or deletes one |
| `link_issues`, `delete_issue_link` | Links two issues, or removes the link |
| `add_remote_link`, `edit_remote_link`, `delete_remote_link`, `delete_remote_link_by_global_id` | Links an issue to an outside page, changes or removes the link |

Boards and sprints, from Jira Software's Agile API:

| Tool | Scope | Does |
|---|---|---|
| `list_boards`, `get_board`, `get_board_configuration` | `read` | Boards, their columns, filter and estimation |
| `list_sprints`, `get_sprint` | `read` | Sprints of a Scrum board |
| `list_board_issues`, `get_backlog`, `list_sprint_issues` | `read` | Issues of a board, its backlog or a sprint, in board order |
| `get_epic`, `list_epic_issues` | `read` | An epic and its issues |
| `create_sprint` | `agile:write` | Creates a future sprint on a Scrum board |
| `update_sprint` | `agile:write` | Renames, dates, starts or completes a sprint |
| `delete_sprint` | `agile:write` | Deletes a sprint that has not started |
| `move_issues_to_sprint`, `move_issues_to_backlog` | `agile:write` | Plans issues in a sprint, or takes them out |
| `rank_issues` | `agile:write` | Moves issues before or after another in the board order |

Comments and work logs:

| Tool | Scope | Does |
|---|---|---|
| `add_comment`, `edit_comment`, `delete_comment` | `comments:write` | Comments on an issue, changes the text of a comment, or deletes it |
| `add_worklog`, `edit_worklog`, `delete_worklog` | `worklogs:write` | Logs work on an issue, changes or deletes a work log; the time tracking follows |

Saved filters:

| Tool | Scope | Does |
|---|---|---|
| `create_filter`, `update_filter`, `delete_filter` | `filters:write` | Saves, changes or deletes a filter, its shares and whether it is a favourite |

Versions and components of a project:

| Tool | Scope | Does |
|---|---|---|
| `list_versions`, `get_version` | `read` | Versions of a project, or one by id |
| `list_components`, `get_component` | `read` | Components of a project, or one by id |
| `create_version`, `update_version`, `delete_version` | `projects:write` | Adds, changes (dates, released, archived) or deletes a version, in a project the user administers |
| `create_component`, `update_component`, `delete_component` | `projects:write` | Adds, changes (lead, default assignee) or deletes a component, in a project the user administers |

## Connecting Claude Code

```sh
claude mcp add --transport http solstice https://solstice.example.org/mcp
```

Then run `/mcp` in Claude Code and choose the server to sign in through the
browser. With a personal access token instead, add
`--header "Authorization: Bearer sol_example"`.

Add `--scope user` to use it in every project. `/mcp` in Claude Code then
lists the server and its tools.

## Connecting VS Code

In `.vscode/mcp.json`, so that VS Code asks for the token instead of storing
it in the file:

```json
{
  "inputs": [
    { "type": "promptString", "id": "solstice-token", "description": "Solstice access token", "password": true }
  ],
  "servers": {
    "solstice": {
      "type": "http",
      "url": "https://solstice.example.org/mcp",
      "headers": { "Authorization": "Bearer ${input:solstice-token}" }
    }
  }
}
```

Other clients need the address `https://solstice.example.org/mcp`, and,
unless they support OAuth, the header `Authorization: Bearer <token>`.

## Behind a reverse proxy

Nothing to add: `/mcp` answers each POST with JSON and keeps no stream open,
so the `location /` of [reverse-proxy.md](reverse-proxy.md) covers it, as it
covers `/oauth/` and `/.well-known/`. The proxy must pass the `Authorization`
header on, which it does by default. The addresses in the metadata, the
issuer included, come from `SOLSTICE_HTTP_BASEURL` when it is set, otherwise
from the request: set it behind a proxy, or clients may refuse the issuer.
