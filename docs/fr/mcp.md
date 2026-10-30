# Point d'accès MCP

Solstice intègre un serveur [Model Context Protocol](https://modelcontextprotocol.io)
à l'adresse `/mcp`, pour que des agents d'IA (Claude Code, assistants des
IDE) puissent chercher et lire des tickets. Il parle le transport Streamable
HTTP, sans session : chaque requête porte ses propres identifiants et reçoit
une réponse JSON.

Les exemples utilisent l'hôte `solstice.example.org` et le jeton `sol_example`.

## Connexion

Le point d'accès n'accepte qu'un jeton d'accès personnel, dans l'en-tête
`Authorization: Bearer sol_example`. Les mots de passe, l'authentification
Basic, les cookies de session et les jetons passés dans l'adresse sont
refusés.

Créez un jeton dans « Mes paramètres », « Jetons d'accès », avec la portée
`read`, et `agile:write` si l'agent doit planifier les sprints. Seules les personnes que la politique des jetons de l'administrateur
autorise peuvent en créer (voir [administration.md](administration.md)). Un
jeton continue de fonctionner si la politique change ensuite, comme sur l'API
REST.

Une requête sans jeton valide reçoit un 401 avec
`WWW-Authenticate: Bearer resource_metadata="https://solstice.example.org/.well-known/oauth-protected-resource"`.
Ce document (RFC 9728) nomme la ressource et les portées que peuvent avoir les
jetons.

## Ce que l'agent peut faire

Un agent voit et lit ce que le propriétaire du jeton peut voir dans
l'interface : mêmes permissions, mêmes niveaux de sécurité, mêmes
commentaires restreints. Un ticket sécurisé que le propriétaire ne peut pas
voir reste caché, dans les recherches comme par sa clé.

Les outils répondent dans le JSON de l'API REST de Jira Server (voir
[rest-api.md](rest-api.md)). Chacun demande une portée ; la liste des outils
ne montre que ceux que les portées du jeton permettent, et appeler un autre
outil échoue en nommant la portée qui manque. Tous les outils de ce tableau
demandent `read`.

| Outil | Donne |
|---|---|
| `search_issues` | Les tickets qui répondent à une requête JQL, page par page |
| `get_issue` | Un ticket, ses champs, commentaires, temps passé, liens, pièces jointes |
| `pick_issues` | Les tickets dont la clé ou le résumé contient un texte |
| `get_transitions` | Les transitions que l'utilisateur peut faire sur un ticket |
| `list_comments`, `get_comment` | Les commentaires d'un ticket |
| `list_worklogs`, `get_worklog` | Le temps passé sur un ticket |
| `get_watchers` | Qui suit un ticket |
| `list_remote_links`, `get_remote_link` | Les liens d'un ticket vers des pages extérieures |
| `get_attachment` | Les informations d'une pièce jointe et son adresse de téléchargement |
| `get_create_meta`, `get_edit_meta` | Les champs qu'on peut remplir à la création ou à la modification |
| `get_my_permissions` | Les permissions de l'utilisateur, globales, dans un projet ou sur un ticket |
| `list_projects`, `get_project`, `get_project_statuses` | Les projets et leurs statuts |
| `get_myself`, `get_user`, `search_users`, `search_assignable_users` | Les utilisateurs |
| `search_groups`, `list_group_members` | Les groupes et leurs membres |
| `list_fields`, `list_issue_types`, `list_statuses`, `list_priorities`, `list_resolutions`, `list_issue_link_types` | Les catalogues de l'instance |
| `get_server_info` | La version et l'adresse du serveur |
| `list_favourite_filters`, `get_filter` | Les filtres enregistrés : les favoris de l'utilisateur, ou un filtre par son id |

Les tableaux et les sprints, de l'API Agile de Jira Software :

| Outil | Portée | Fait |
|---|---|---|
| `list_boards`, `get_board`, `get_board_configuration` | `read` | Les tableaux, leurs colonnes, leur filtre et leur estimation |
| `list_sprints`, `get_sprint` | `read` | Les sprints d'un tableau Scrum |
| `list_board_issues`, `get_backlog`, `list_sprint_issues` | `read` | Les tickets d'un tableau, de son backlog ou d'un sprint, dans l'ordre du tableau |
| `get_epic`, `list_epic_issues` | `read` | Une epic et ses tickets |
| `update_sprint` | `agile:write` | Renomme un sprint, change ses dates, le démarre ou le termine |
| `delete_sprint` | `agile:write` | Supprime un sprint pas encore démarré |
| `move_issues_to_sprint`, `move_issues_to_backlog` | `agile:write` | Planifie des tickets dans un sprint, ou les en retire |
| `rank_issues` | `agile:write` | Place des tickets avant ou après un autre dans l'ordre du tableau |

Les filtres enregistrés :

| Outil | Portée | Fait |
|---|---|---|
| `create_filter`, `update_filter`, `delete_filter` | `filters:write` | Enregistre, modifie ou supprime un filtre, ses partages et s'il est en favori |

Les versions et composants, et les outils qui modifient les tickets, ne sont
pas encore proposés.

## Connecter Claude Code

```sh
claude mcp add --transport http solstice https://solstice.example.org/mcp \
  --header "Authorization: Bearer sol_example"
```

Ajoutez `--scope user` pour l'utiliser dans tous les projets. `/mcp` dans
Claude Code liste ensuite le serveur et ses outils.

## Connecter VS Code

Dans `.vscode/mcp.json`, pour que VS Code demande le jeton au lieu de
l'enregistrer dans le fichier :

```json
{
  "inputs": [
    { "type": "promptString", "id": "solstice-token", "description": "Jeton d'accès Solstice", "password": true }
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

Les autres clients ont besoin des deux mêmes choses : l'adresse
`https://solstice.example.org/mcp` et l'en-tête `Authorization: Bearer <jeton>`.

## Derrière un proxy inverse

Rien à ajouter : `/mcp` répond à chaque POST en JSON et ne garde aucun flux
ouvert, donc le `location /` de [reverse-proxy.md](reverse-proxy.md) suffit.
Le proxy doit transmettre l'en-tête `Authorization`, ce qu'il fait par défaut.
