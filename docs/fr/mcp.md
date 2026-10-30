# Point d'accès MCP

Solstice intègre un serveur [Model Context Protocol](https://modelcontextprotocol.io)
à l'adresse `/mcp`, pour que des agents d'IA (Claude Code, assistants des
IDE) puissent chercher, lire et modifier des tickets. Il parle le transport Streamable
HTTP, sans session : chaque requête porte ses propres identifiants et reçoit
une réponse JSON.

Les exemples utilisent l'hôte `solstice.example.org` et le jeton `sol_example`.

## Connexion

Le point d'accès accepte un jeton porteur, dans l'en-tête
`Authorization: Bearer …` : un jeton d'accès OAuth que le client obtient en
vous faisant vous connecter (ci-dessous), ou un jeton d'accès personnel. Les
mots de passe, l'authentification Basic, les cookies de session et les jetons
passés dans l'adresse sont refusés.

### Avec OAuth

Les clients qui gèrent l'autorisation MCP (connecteurs de claude.ai, Claude
Code, certains IDE) n'ont besoin que de l'adresse
`https://solstice.example.org/mcp`. À la première requête, ils ouvrent le
navigateur sur Solstice : vous vous connectez comme d'habitude (mot de passe,
LDAP ou authentification unique), puis une page de consentement nomme
l'application, l'adresse où elle revient et les portées qu'elle demande.
Décochez ce dont elle n'a pas besoin, puis autorisez ou refusez. `read` est
toujours accordée ; `admin` ne l'est jamais, et aucun outil n'en a besoin.

Solstice est le serveur d'autorisation OAuth 2.1 de `/mcp`, comme le décrit la
[spécification d'autorisation MCP](https://modelcontextprotocol.io/specification/latest/basic/authorization) :

- ses métadonnées (RFC 8414) sont à l'adresse
  `https://solstice.example.org/.well-known/oauth-authorization-server` ;
- le flux du code d'autorisation, avec PKCE (`S256` seulement), le paramètre
  `iss` dans la réponse (RFC 9207), et le paramètre `resource` obligatoire et
  égal à `https://solstice.example.org/mcp` (RFC 8707) ;
- les clients sont identifiés par un Client ID Metadata Document : leur
  `client_id` est une adresse HTTPS qui sert un document JSON avec
  `client_id`, `client_name` et `redirect_uris`. Solstice le lit au moment où
  vous autorisez, sans suivre de redirection et jamais depuis une adresse
  privée ou de bouclage ;
- les clients sans un tel document s'enregistrent par Dynamic Client
  Registration (RFC 7591), à `https://solstice.example.org/register`, que les
  métadonnées nomment `registration_endpoint`. Seulement des clients publics
  (`token_endpoint_auth_method` `none`), avec un `client_name` et des
  adresses de retour en HTTPS ou sur une adresse de bouclage
  (`http://127.0.0.1:…`, `http://localhost:…`, tout port). Une adresse peut
  enregistrer 10 clients par heure. Un client que personne n'autorise dans la
  journée est supprimé. Son nom n'engage que lui : la page de consentement le
  dit au lieu de nommer un hôte. Un administrateur peut désactiver
  l'enregistrement ; les clients déjà enregistrés continuent de fonctionner ;
- les jetons d'accès (`solo_…`) durent une heure. Les jetons de
  rafraîchissement (`solor_…`) sont remplacés à chaque usage ; un jeton
  utilisé deux fois révoque tous ceux qu'il a donnés. Un jeton de
  rafraîchissement inutilisé pendant 30 jours expire (un administrateur peut
  changer ce nombre de jours, à côté de la politique des jetons), et le client
  vous redemande l'autorisation ;
- ces jetons ne servent que sur `/mcp` : l'API de l'interface et l'API REST
  les refusent.

Qui peut autoriser une application suit la politique des jetons, comme pour
les jetons d'accès personnels (voir [administration.md](administration.md)) ;
une application déjà autorisée continue de fonctionner si la politique change.
Les enregistrements, autorisations, refus et révocations vont dans le journal
d'audit, et les administrateurs peuvent lister et révoquer toutes les
autorisations.

« Mes paramètres », « Applications connectées » liste les applications que
vous avez autorisées, avec leurs portées et leur dernière utilisation. En
révoquer une met fin à ses jetons aussitôt. Changer votre mot de passe, ou la
désactivation de votre compte, les révoque toutes.

### Avec un jeton d'accès personnel

L'en-tête est `Authorization: Bearer sol_example`. Créez un jeton dans « Mes paramètres », « Jetons d'accès », avec la portée
`read`, plus `issues:write` si l'agent doit modifier des tickets,
`comments:write` pour commenter, `worklogs:write` pour saisir du temps passé et
`agile:write` pour planifier les sprints. Seules les personnes que la politique des jetons de l'administrateur
autorise peuvent en créer (voir [administration.md](administration.md)). Un
jeton continue de fonctionner si la politique change ensuite, comme sur l'API
REST.

Une requête sans jeton valide reçoit un 401 avec
`WWW-Authenticate: Bearer resource_metadata="https://solstice.example.org/.well-known/oauth-protected-resource"`.
Ce document (RFC 9728) nomme la ressource, son serveur d'autorisation et les
portées que peuvent avoir les jetons.

## Ce que l'agent peut faire

Un agent voit et fait ce que le propriétaire du jeton peut voir et faire
dans l'interface : mêmes permissions, mêmes workflows, mêmes niveaux de
sécurité, mêmes commentaires restreints, et l'historique attribue les
modifications au propriétaire. Un ticket sécurisé que le propriétaire ne peut pas
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

Ces outils demandent `issues:write` :

| Outil | Fait |
|---|---|
| `create_issue`, `edit_issue`, `delete_issue` | Crée, modifie ou supprime un ticket |
| `assign_issue` | Attribue un ticket, ou retire son assigné |
| `transition_issue` | Fait avancer un ticket dans son workflow |
| `add_watcher`, `remove_watcher` | Fait suivre un ticket à quelqu'un, ou arrête |
| `add_vote`, `remove_vote` | Vote pour un ticket, ou retire le vote |
| `add_attachment`, `delete_attachment` | Joint un fichier (encodé en base64, jusqu'à la limite d'envoi), ou en supprime un |
| `link_issues`, `delete_issue_link` | Lie deux tickets, ou supprime le lien |
| `add_remote_link`, `edit_remote_link`, `delete_remote_link`, `delete_remote_link_by_global_id` | Lie un ticket à une page extérieure, modifie ou supprime le lien |

Les tableaux et les sprints, de l'API Agile de Jira Software :

| Outil | Portée | Fait |
|---|---|---|
| `list_boards`, `get_board`, `get_board_configuration` | `read` | Les tableaux, leurs colonnes, leur filtre et leur estimation |
| `list_sprints`, `get_sprint` | `read` | Les sprints d'un tableau Scrum |
| `list_board_issues`, `get_backlog`, `list_sprint_issues` | `read` | Les tickets d'un tableau, de son backlog ou d'un sprint, dans l'ordre du tableau |
| `get_epic`, `list_epic_issues` | `read` | Une epic et ses tickets |
| `create_sprint` | `agile:write` | Crée un sprint futur sur un tableau Scrum |
| `update_sprint` | `agile:write` | Renomme un sprint, change ses dates, le démarre ou le termine |
| `delete_sprint` | `agile:write` | Supprime un sprint pas encore démarré |
| `move_issues_to_sprint`, `move_issues_to_backlog` | `agile:write` | Planifie des tickets dans un sprint, ou les en retire |
| `rank_issues` | `agile:write` | Place des tickets avant ou après un autre dans l'ordre du tableau |

Les commentaires et le temps passé :

| Outil | Portée | Fait |
|---|---|---|
| `add_comment`, `edit_comment`, `delete_comment` | `comments:write` | Commente un ticket, modifie le texte d'un commentaire, ou le supprime |
| `add_worklog`, `edit_worklog`, `delete_worklog` | `worklogs:write` | Saisit du temps passé sur un ticket, modifie ou supprime une saisie ; le suivi du temps suit |

Les filtres enregistrés :

| Outil | Portée | Fait |
|---|---|---|
| `create_filter`, `update_filter`, `delete_filter` | `filters:write` | Enregistre, modifie ou supprime un filtre, ses partages et s'il est en favori |

Les versions et composants d'un projet :

| Outil | Portée | Fait |
|---|---|---|
| `list_versions`, `get_version` | `read` | Les versions d'un projet, ou une seule par son id |
| `list_components`, `get_component` | `read` | Les composants d'un projet, ou un seul par son id |
| `create_version`, `update_version`, `delete_version` | `projects:write` | Ajoute, modifie (dates, livrée, archivée) ou supprime une version, dans un projet que l'utilisateur administre |
| `create_component`, `update_component`, `delete_component` | `projects:write` | Ajoute, modifie (responsable, assigné par défaut) ou supprime un composant, dans un projet que l'utilisateur administre |

## Connecter Claude Code

```sh
claude mcp add --transport http solstice https://solstice.example.org/mcp
```

Lancez ensuite `/mcp` dans Claude Code et choisissez le serveur pour vous
connecter dans le navigateur. Avec un jeton d'accès personnel, ajoutez plutôt
`--header "Authorization: Bearer sol_example"`.

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

Les autres clients ont besoin de l'adresse `https://solstice.example.org/mcp`
et, s'ils ne gèrent pas OAuth, de l'en-tête `Authorization: Bearer <jeton>`.

## Derrière un proxy inverse

Rien à ajouter : `/mcp` répond à chaque POST en JSON et ne garde aucun flux
ouvert, donc le `location /` de [reverse-proxy.md](reverse-proxy.md) suffit,
comme pour `/oauth/` et `/.well-known/`. Le proxy doit transmettre l'en-tête
`Authorization`, ce qu'il fait par défaut. Les adresses des métadonnées,
émetteur compris, viennent de `SOLSTICE_HTTP_BASEURL` s'il est défini, sinon
de la requête : définissez-le derrière un proxy, ou des clients pourraient
refuser l'émetteur.
