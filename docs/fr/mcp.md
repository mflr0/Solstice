# Point d'accès MCP

Solstice intègre un serveur [Model Context Protocol](https://modelcontextprotocol.io)
à l'adresse `/mcp`, pour que des agents d'IA (Claude Code, assistants des
IDE) puissent chercher, lire et modifier des tickets. Il parle le transport Streamable
HTTP, sans session : chaque requête porte ses propres identifiants et reçoit
une réponse JSON.

Les exemples utilisent l'hôte `solstice.example.org` et le jeton `sol_example`.

## Connexion

Le point d'accès n'accepte qu'un jeton d'accès personnel, dans l'en-tête
`Authorization: Bearer sol_example`. Les mots de passe, l'authentification
Basic, les cookies de session et les jetons passés dans l'adresse sont
refusés.

Créez un jeton dans « Mes paramètres », « Jetons d'accès », avec la portée
`read`, et `issues:write` pour que l'agent puisse modifier des tickets. Seules les personnes que la politique des jetons de l'administrateur
autorise peuvent en créer (voir [administration.md](administration.md)). Un
jeton continue de fonctionner si la politique change ensuite, comme sur l'API
REST.

Une requête sans jeton valide reçoit un 401 avec
`WWW-Authenticate: Bearer resource_metadata="https://solstice.example.org/.well-known/oauth-protected-resource"`.
Ce document (RFC 9728) nomme la ressource et les portées que peuvent avoir les
jetons.

## Ce que l'agent peut faire

Un agent voit et fait ce que le propriétaire du jeton peut voir et faire
dans l'interface : mêmes permissions, mêmes workflows, mêmes niveaux de
sécurité, mêmes commentaires restreints, et l'historique attribue les
modifications au propriétaire. Un ticket sécurisé que le propriétaire ne peut pas
voir reste caché, dans les recherches comme par sa clé.

Les outils répondent dans le JSON de l'API REST de Jira Server (voir
[rest-api.md](rest-api.md)). Chacun demande une portée ; la liste des outils
ne montre que ceux que les portées du jeton permettent, et appeler un autre
outil échoue en nommant la portée qui manque.

Ces outils demandent `read` :

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

Ces outils demandent `issues:write` :

| Outil | Fait |
|---|---|
| `create_issue`, `edit_issue`, `delete_issue` | Crée, modifie ou supprime un ticket |
| `assign_issue` | Attribue un ticket, ou retire son responsable |
| `transition_issue` | Fait avancer un ticket dans son workflow |
| `add_watcher`, `remove_watcher` | Fait suivre un ticket à quelqu'un, ou arrête |
| `add_vote`, `remove_vote` | Vote pour un ticket, ou retire le vote |
| `add_attachment`, `delete_attachment` | Joint un fichier (encodé en base64, jusqu'à la limite d'envoi), ou en supprime un |
| `link_issues`, `delete_issue_link` | Lie deux tickets, ou supprime le lien |
| `add_remote_link`, `edit_remote_link`, `delete_remote_link`, `delete_remote_link_by_global_id` | Lie un ticket à une page extérieure, modifie ou supprime le lien |

Les outils qui modifient les commentaires ou le temps passé, et les
tableaux, sprints, filtres, versions et composants, ne sont pas encore
proposés.

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
