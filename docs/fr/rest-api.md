# API REST

Solstice répond à une partie des API REST de Jira Server et Data Center, pour
que les scripts, les tâches d'intégration continue et les extensions d'IDE
écrits pour Jira continuent de fonctionner :

- `/rest/api/2` et son alias `/rest/api/latest`
- `/rest/agile/1.0` et son alias `/rest/agile/latest`

La version 3 de l'API de Jira n'est pas proposée. Le serveur se présente comme
Jira 9.12.0 sur `serverInfo`, avec `serverTitle` à `Solstice`, pour que les
clients qui testent la version l'acceptent.

Les exemples utilisent l'hôte `solstice.example.org` et le jeton
`sol_example`. Les vrais jetons sont plus longs.

L'interface a sa propre API sous `/api`. Elle n'est pas compatible avec Jira
et n'est pas décrite ici.

## Authentification

Toutes les routes, sauf `GET /rest/api/2/serverInfo`, demandent un utilisateur
connecté. Sans identifiants, la réponse est 401.

| Méthode | Comment |
|---|---|
| Jeton d'accès personnel | `Authorization: Bearer sol_example` |
| Basic, avec un jeton | `Authorization: Basic …` avec l'identifiant du compte et le jeton comme mot de passe |
| Basic, avec un mot de passe | L'identifiant et le mot de passe du compte. Les échecs sont limités comme sur la page de connexion, et la réponse est 429 quand la limite est atteinte. |
| Cookie de session | Le cookie que l'interface pose à la connexion, ou celui que répond `POST /rest/auth/1/session` (voir plus bas). |

Si une requête porte un en-tête `Authorization`, il est utilisé et le cookie
est ignoré. Préférez les jetons aux mots de passe : un jeton se révoque
isolément, et deviner le mot de passe de quelqu'un ne peut pas bloquer ses
scripts.

Créez un jeton dans « Mes réglages », « Jetons d'accès ». Le secret n'est
affiché qu'une fois. Seules les personnes autorisées par la politique de
l'administrateur peuvent créer des jetons (voir
[administration.md](administration.md)), et une requête connectée avec un jeton
ne peut pas en créer un autre.

Chaque jeton a des portées, choisies à sa création. Une requête connectée avec
un jeton ne peut utiliser que les routes que ses portées couvrent ; sinon elle
répond 403 avec
`WWW-Authenticate: Bearer error="insufficient_scope", scope="<portée requise>"`.
Les mots de passe et les cookies de session ont toutes les portées.

| Portée | Couvre |
|---|---|
| `read` | Tous les `GET`, et `POST /rest/api/2/search`. Toute autre portée l'implique. |
| `issues:write` | Créer, modifier, supprimer, déplacer, cloner, faire avancer et assigner des tickets ; liens, liens distants, pièces jointes, votes, observateurs. |
| `comments:write` | Les commentaires. |
| `worklogs:write` | Le temps passé. |
| `agile:write` | Tableaux, sprints, rang (`/rest/agile/1.0`). |
| `filters:write` | Filtres et favoris. |
| `projects:write` | Versions et composants. |
| `profile:write` | Le profil, l'avatar et les réglages de l'utilisateur. |
| `admin` | L'administration. Seul un administrateur peut créer un jeton qui l'a. |

Une portée n'élargit jamais les permissions de l'utilisateur : un jeton
`issues:write` ne peut pas modifier un ticket que son utilisateur ne peut pas
modifier.

```sh
curl -H "Authorization: Bearer sol_example" \
  https://solstice.example.org/rest/api/2/myself
```

Une requête voit et fait ce que son utilisateur peut voir et faire dans
l'interface : les mêmes permissions, commentaires restreints, niveaux de
confidentialité et historique s'appliquent. Ce que l'utilisateur ne peut pas
voir répond 404 ; ce qui est visible mais non permis répond 403.

### Écritures : la règle de l'en-tête anti-falsification

Une requête qui modifie quelque chose (autre que GET, HEAD ou OPTIONS) doit
prouver qu'elle ne peut pas avoir été forgée par un autre site. Elle est
acceptée quand l'une de ces conditions est remplie :

- Elle porte l'en-tête `X-Atlassian-Token: no-check`, ou l'en-tête
  `X-Solstice` avec une valeur quelconque.
- Elle est connectée par un jeton Bearer. Les navigateurs n'en ajoutent jamais
  d'eux-mêmes.
- Elle est connectée par des identifiants Basic, et ce n'est pas un POST dont
  le corps est d'un type qu'un formulaire HTML peut envoyer : pas de
  `Content-Type`, `application/x-www-form-urlencoded`, `multipart/form-data`
  ou `text/plain`.

Une requête connectée par le cookie de session doit toujours porter l'un des
deux en-têtes. Sinon la réponse est 403.

En pratique, un script avec un jeton n'a besoin d'aucun en-tête
supplémentaire. Un script avec des identifiants Basic a besoin de
`X-Atlassian-Token: no-check` pour téléverser une pièce jointe, qui est un POST
multipart :

```sh
curl -u alice:sol_example \
  -H "X-Atlassian-Token: no-check" \
  -F "file=@report.pdf" \
  https://solstice.example.org/rest/api/2/issue/DEMO-1/attachments
```

### La connexion par cookie de Jira

Pour les scripts écrits pour la connexion par cookie de Jira :

| Requête | Remarques |
|---|---|
| `POST /rest/auth/1/session` | `{"username": "alice", "password": "…"}`. Répond `session`, le `name` et la `value` du cookie, et `loginInfo`. Les échecs sont limités comme sur la page de connexion. |
| `GET /rest/auth/1/session` | Le `name` de l'utilisateur connecté et `loginInfo`. |
| `DELETE /rest/auth/1/session` | Déconnecte. 204. |

Un POST avec un corps JSON et un DELETE n'ont pas besoin ici d'en-tête
anti-falsification : un navigateur ne peut envoyer ni l'un ni l'autre à un
autre site sans le lui demander d'abord, et Solstice n'accepte jamais. Un POST
dont le corps est d'un type qu'un formulaire HTML peut envoyer en a toujours
besoin.

## Conventions

- Les corps sont en JSON. Les champs inconnus d'un corps de requête sont
  ignorés, car les clients Jira en envoient beaucoup dont Solstice n'a pas
  l'usage. Les versions, composants et filtres sont plus stricts : un champ
  qui ne peut pas être renseigné répond 400, sauf ceux en lecture seule que
  Jira renvoie lui-même (`self`, `id`, …), qui sont ignorés.
- Les erreurs suivent le format de Jira :
  `{"errorMessages": ["…"], "errors": {"champ": "…"}}`. Les messages sont dans
  la langue de la requête (`Accept-Language`), sinon celle de l'utilisateur,
  sinon la langue par défaut de l'instance.
- Les dates s'écrivent comme dans Jira : `2026-10-15T09:12:00.000+0200`, dans
  le fuseau horaire de l'instance. Les requêtes acceptent aussi le RFC 3339.
- La pagination utilise `startAt` et `maxResults`. Valeurs par défaut et
  maximales : 50 et 1000 pour la recherche, les listes d'utilisateurs et les
  tickets d'un tableau, d'un backlog, d'un sprint ou d'une épopée ; 50 et 50
  pour les tableaux et les sprints ; 5000 pour les commentaires et les saisies
  de temps. `maxResults=0` ne fait que compter.
- Un ticket se désigne par sa clé (`DEMO-1`) ou par son identifiant numérique.
  Une ancienne clé de projet est toujours résolue.
- Une requête vers une adresse sous `/rest/` qui n'est pas listée ci-dessous
  répond 404.

## Tickets

| Requête | Remarques |
|---|---|
| `POST /rest/api/2/issue` | Crée un ticket. 201 avec `id`, `key` et `self`. |
| `GET /rest/api/2/issue/{key}` | Paramètres `fields` et `expand`. Tous les champs par défaut. |
| `PUT /rest/api/2/issue/{key}` | Modifie avec `fields` et `update`. 204. |
| `DELETE /rest/api/2/issue/{key}` | Refusé si le ticket a des sous-tâches, sauf `?deleteSubtasks=true`. 204. |
| `PUT /rest/api/2/issue/{key}/assignee` | `{"name": "alice"}`, `{"name": null}` pour personne, `{"name": "-1"}` pour l'assigné par défaut du projet. 204. |
| `GET /rest/api/2/issue/{key}/watchers` | Le nombre pour tous, les noms pour ceux qui gèrent les observateurs. |
| `POST /rest/api/2/issue/{key}/watchers` | Le corps est un identifiant sous forme de chaîne JSON ; vide, c'est vous. |
| `DELETE /rest/api/2/issue/{key}/watchers` | `?username=` ; vide, c'est vous. |
| `POST /rest/api/2/issue/{key}/votes` | Voter. 204. |
| `DELETE /rest/api/2/issue/{key}/votes` | Retirer son vote. 204. |
| `POST /rest/api/2/issueLink` | Lie deux tickets. `inwardIssue` est la source : `{"type": {"name": "Blocks"}, "inwardIssue": {"key": "A-1"}, "outwardIssue": {"key": "A-2"}}` signifie « A-1 bloque A-2 ». Un `comment` facultatif est ajouté au ticket source. 201. |
| `DELETE /rest/api/2/issueLink/{id}` | Retire un lien. |
| `GET /rest/api/2/issueLinkType` | Les types de lien. |
| `GET /rest/api/2/issue/createmeta` | Les projets et types de ticket où l'utilisateur peut créer des tickets. `projectKeys`, `projectIds`, `issuetypeIds` et `issuetypeNames` restreignent la liste. `expand=projects.issuetypes.fields` ajoute les champs de chaque type, avec leurs valeurs permises. |
| `GET /rest/api/2/issue/{key}/editmeta` | Les champs qui peuvent être renseignés sur le ticket, avec leurs valeurs permises et leurs opérations. |
| `GET /rest/api/2/issue/picker` | Les tickets dont la clé ou le résumé correspond à `query`, en deux sections : `hs`, les tickets que l'utilisateur a modifiés ou commentés en dernier (Solstice n'enregistre pas les tickets que les gens ont consultés), et `cs`, la recherche en cours, restreinte par `currentJQL`. `currentIssueKey`, `showSubTasks=false` et `showSubTaskParent=false` écartent des tickets. |

`expand` sur un ticket accepte `changelog`, `renderedFields`, `names` et
`transitions` (avec `transitions.fields`).

Les champs que Solstice sait renseigner à la création et à la modification :
`project`, `issuetype`, `summary`, `description`, `environment`, `duedate`,
`timetracking` (`originalEstimate`, `remainingEstimate`), `labels`,
`priority`, `resolution`, `assignee`, `security`, `components`, `fixVersions`,
`versions`, `parent` (sous-tâches, à la création), les champs personnalisés
sous la forme `customfield_<id>`, et les champs de sprint et de lien d'épopée
de Jira Software. Dans `update`, les opérations sont `set`, `edit`, `add` et
`remove` (`add` et `remove` pour les étiquettes, composants et versions), et
`{"comment": [{"add": {"body": "…"}}]}` ajoute un commentaire.

Limites :

- Un ticket ne peut pas être déplacé vers un autre projet ni vers un autre
  parent par l'API.
- `reporter` ne peut désigner que l'utilisateur de la requête (à la création)
  ou le rapporteur actuel. Solstice enregistre qui a réellement créé un ticket.
- Un champ inconnu de Solstice répond 400 avec le nom du champ.

Les valeurs des champs personnalisés suivent les formes de Jira : chaînes,
nombres, `{"id": "…"}` ou `{"value": "…"}` pour les options, listes pour les
choix multiples, identifiants pour les champs d'utilisateur. Un champ hors du
contexte de son projet et de son type de ticket est refusé.

## Recherche et JQL

| Requête | Remarques |
|---|---|
| `GET /rest/api/2/search` | Paramètres `jql`, `startAt`, `maxResults`, `fields`, `expand`. |
| `POST /rest/api/2/search` | Les mêmes dans un corps JSON. |

`fields` vaut `*navigable` par défaut. La réponse contient `startAt`,
`maxResults`, `total` et `issues`, et `names` avec `expand=names`.

Solstice implémente lui-même le langage de requête de Jira (JQL). Ce qui
fonctionne :

- **Logique.** `AND`, `OR`, `NOT`, parenthèses.
- **Opérateurs.** `=`, `!=`, `>`, `>=`, `<`, `<=`, `~`, `!~`, `IN`, `NOT IN`,
  `IS EMPTY`, `IS NOT EMPTY` (aussi `NULL`).
- **Champs.** `project`, `status`, `statusCategory`, `type` (`issuetype`),
  `priority`, `resolution`, `assignee`, `reporter`, `creator`, `labels`,
  `component`, `fixVersion`, `affectedVersion`, `sprint`, `"Epic Link"`,
  `parent`, `level`, `watcher`, `commenter`, `voter`, `created`, `updated`,
  `resolved`, `due`, `key` (`issue`, `id`), `summary`, `description`,
  `environment`, `text`, `comment`, `timespent`, `originalEstimate`,
  `remainingEstimate`, et les champs personnalisés par leur nom ou sous la
  forme `cf[10001]`. `participant`, `participants` et `involves` sont une
  extension de Solstice : tous les rôles qu'une personne peut avoir sur un
  ticket.
- **Fonctions.** `currentUser()`, `membersOf("groupe")`, `now()`,
  `startOfDay()`, `endOfDay()`, `startOfWeek()`, `endOfWeek()`,
  `startOfMonth()`, `endOfMonth()`, `startOfYear()`, `endOfYear()` (chacune
  avec un décalage facultatif comme `"-1d"`), `openSprints()`,
  `closedSprints()`, `futureSprints()`, `releasedVersions()`,
  `unreleasedVersions()`, `earliestUnreleasedVersion()`,
  `latestReleasedVersion()` (chacune avec des projets en arguments
  facultatifs), `standardIssueTypes()`, `subtaskIssueTypes()`.
- **Dates.** `2026-10-15`, `"2026-10-15 14:30"`, les mêmes avec `/`, et des
  valeurs relatives comme `-7d`, `2w`, `-1M` (unités `y`, `M`, `w`, `d`, `h`,
  `m`). Elles sont lues dans le fuseau horaire de l'instance. Les semaines
  commencent le lundi.
- **Durées.** `2h`, `1d 4h`, `1w`. Un jour compte 8 heures et une semaine 5
  jours.
- **Historique.** `WAS`, `WAS NOT`, `WAS IN`, `WAS NOT IN` et `CHANGED`, avec
  `AFTER`, `BEFORE`, `ON`, `DURING`, `BY`, `FROM` et `TO`.
- **Tri.** `ORDER BY` sur `key`, `created`, `updated`, `resolved`, `due`,
  `priority`, `status`, `summary`, `assignee`, `reporter`, `project`, `type`,
  `rank`, `fixVersion`, `affectedVersion`, `component`, `resolution`, les
  estimations, `timespent`, `"Epic Link"`, `sprint` et les champs
  personnalisés. Sans `ORDER BY`, les tickets modifiés le plus récemment
  viennent en premier.
- `resolution = Unresolved` trouve les tickets sans résolution.

Limites :

- Les clauses d'historique ne portent que sur `status`, `priority`,
  `resolution`, `assignee`, `reporter` et `fixVersion`.
- `rank`, `"Epic Name"`, `"Epic Status"` et `"Epic Color"` ne s'utilisent pas
  comme clauses de recherche. `rank` s'utilise dans `ORDER BY`.
- Une fonction absente de la liste ci-dessus est refusée avec un message
  d'erreur, de même qu'un champ inconnu.
- `issueHistory()` est accepté et ne trouve rien : Solstice n'enregistre pas
  les tickets que les gens ont consultés.
- `commenter` ignore les commentaires restreints.
- Un JQL invalide répond 400 avec la position et la raison. Il n'y a pas
  d'option `validateQuery` ni de point d'accès d'autocomplétion.

## Transitions

| Requête | Remarques |
|---|---|
| `GET /rest/api/2/issue/{key}/transitions` | Les passages que l'utilisateur peut faire maintenant. `?transitionId=` n'en garde qu'un. `expand=transitions.fields` ajoute les champs. |
| `POST /rest/api/2/issue/{key}/transitions` | `{"transition": {"id": "21"}}`, ou par `name`. `fields` et `update` s'appliquent avec le passage. 204. |

Une transition indisponible répond 400 sur `transition`.

## Commentaires

| Requête | Remarques |
|---|---|
| `GET /rest/api/2/issue/{key}/comment` | `startAt`, `maxResults`, `orderBy`. Seulement les commentaires que l'utilisateur peut lire. |
| `POST /rest/api/2/issue/{key}/comment` | `{"body": "…", "visibility": {"type": "group", "value": "staff"}}`. `type` vaut `group` ou `role`. 201. |
| `GET /rest/api/2/issue/{key}/comment/{id}` | |
| `PUT /rest/api/2/issue/{key}/comment/{id}` | |
| `DELETE /rest/api/2/issue/{key}/comment/{id}` | |

## Saisies de temps

| Requête | Remarques |
|---|---|
| `GET /rest/api/2/issue/{key}/worklog` | |
| `POST /rest/api/2/issue/{key}/worklog` | `timeSpent` (`"1h 30m"`) ou `timeSpentSeconds`, `started`, `comment`. 201. |
| `GET /rest/api/2/issue/{key}/worklog/{id}` | |
| `PUT /rest/api/2/issue/{key}/worklog/{id}` | |
| `DELETE /rest/api/2/issue/{key}/worklog/{id}` | |

Limites : les options `adjustEstimate` ne sont pas prises en charge ; le reste
à faire s'ajuste toujours tout seul. Un `visibility` sur une saisie de temps
est refusé.

## Pièces jointes

| Requête | Remarques |
|---|---|
| `POST /rest/api/2/issue/{key}/attachments` | Envoi multipart ; le nom du champ n'a pas d'importance. Répond la liste des pièces ajoutées. Demande `X-Atlassian-Token: no-check` avec une connexion Basic ou par cookie. |
| `GET /rest/api/2/attachment/{id}` | Les métadonnées de la pièce jointe. |
| `DELETE /rest/api/2/attachment/{id}` | |
| `GET /secure/attachment/{id}/{filename}` | Le fichier lui-même, à l'adresse qu'utilise Jira. |

La taille d'un envoi est plafonnée par `SOLSTICE_HTTP_MAXUPLOAD` (100 MiB par
défaut). Une pièce jointe importée sans son fichier répond 404 au
téléchargement.

## Liens distants

Des liens d'un ticket vers une adresse hors de Solstice.

| Requête | Remarques |
|---|---|
| `GET /rest/api/2/issue/{key}/remotelink` | Les liens du ticket. Avec `?globalId=`, ce seul lien. |
| `POST /rest/api/2/issue/{key}/remotelink` | `{"globalId": "…", "relationship": "…", "object": {"url": "https://…", "title": "…"}}`. Seuls `object.url` et `object.title` sont obligatoires. Un `globalId` que le ticket a déjà met à jour ce lien et répond 200 ; sinon 201. |
| `GET /rest/api/2/issue/{key}/remotelink/{linkId}` | |
| `PUT /rest/api/2/issue/{key}/remotelink/{linkId}` | Remplace l'URL, le titre et la relation. Le `globalId` ne peut pas changer. 204. |
| `DELETE /rest/api/2/issue/{key}/remotelink/{linkId}` | 204. |
| `DELETE /rest/api/2/issue/{key}/remotelink?globalId=` | 204. |

Ajouter, modifier et retirer un lien demande la permission « Lier », et
s'inscrit dans l'historique du ticket. Seules les adresses `http`, `https` et
`mailto` sont acceptées. `application`, `icon`, et le `summary` et le `status`
de l'objet sont ignorés.

## Projets, versions et composants

| Requête | Remarques |
|---|---|
| `GET /rest/api/2/project` | Les projets que l'utilisateur peut voir. |
| `GET /rest/api/2/project/{key}` | Par clé, ancienne clé ou identifiant. Inclut les types de ticket, composants et versions. |
| `GET /rest/api/2/project/{key}/statuses` | Les statuts du workflow de chaque type de ticket. |
| `GET /rest/api/2/project/{key}/versions` | |
| `GET /rest/api/2/project/{key}/components` | |
| `GET /rest/api/2/version/{id}` | |
| `POST /rest/api/2/version` | `project` (une clé) ou `projectId`, `name`, `description`, `startDate`, `releaseDate`, `released`, `archived`. 201. |
| `PUT /rest/api/2/version/{id}` | Les mêmes champs, sauf le projet. Seuls les champs donnés changent. |
| `DELETE /rest/api/2/version/{id}` | `moveFixIssuesTo` et `moveAffectedIssuesTo` désignent une autre version du projet vers laquelle déplacer ses tickets ; sinon les tickets la perdent. 204. |
| `GET /rest/api/2/component/{id}` | |
| `POST /rest/api/2/component` | `project` (une clé) ou `projectId`, `name`, `description`, `leadUserName`, `assigneeType` (`PROJECT_DEFAULT`, `COMPONENT_LEAD`, `PROJECT_LEAD` ou `UNASSIGNED`), `archived`. 201. |
| `PUT /rest/api/2/component/{id}` | Les mêmes champs, sauf le projet. Seuls les champs donnés changent. |
| `DELETE /rest/api/2/component/{id}` | `moveIssuesTo` désigne un autre composant du projet vers lequel déplacer ses tickets ; sinon les tickets le perdent. 204. |

Créer, modifier et supprimer des versions et des composants demande
l'administrateur du projet, comme dans l'interface. Les projets sont en
lecture seule dans l'API REST.

## Utilisateurs

| Requête | Remarques |
|---|---|
| `GET /rest/api/2/myself` | L'utilisateur de la requête. |
| `GET /rest/api/2/user?username=` | Ou `?key=`. Les groupes ne sont listés que pour l'utilisateur lui-même et pour les administrateurs. |
| `GET /rest/api/2/user/search?username=` | Cherche dans l'identifiant, le nom affiché et l'adresse e-mail. `.` correspond à tout le monde. `includeActive` et `includeInactive` choisissent les comptes. |
| `GET /rest/api/2/user/assignable/search?project=` | Ou `?issueKey=`. Les utilisateurs actifs à qui l'on peut assigner des tickets. |
| `GET /rest/api/2/mypermissions` | Avec `projectKey`, `projectId`, `issueKey` ou `issueId` ; sinon, dans au moins un projet. |
| `GET /secure/useravatar`, `GET /secure/projectavatar` | Les avatars sont générés : des initiales sur une couleur. |
| `GET /rest/api/2/groups/picker?query=` | Les groupes dont le nom contient `query`. `exclude` (répétable) et `maxResults`. Les administrateurs voient tous les groupes, les autres seulement ceux dont ils font partie. |
| `GET /rest/api/2/group/member?groupname=` | Les membres d'un groupe, pour les administrateurs seulement. `includeInactiveUsers`, `startAt`, `maxResults` (50 au plus). |

On ne peut pas créer, modifier ni supprimer d'utilisateurs ni de groupes par
l'API. `mypermissions` associe les noms de permissions de Jira à ceux de
Solstice, qui sont moins nombreux et plus larges.

## Métadonnées

| Requête | Remarques |
|---|---|
| `GET /rest/api/2/serverInfo` | Sans connexion. |
| `GET /rest/api/2/field` | Les champs système et personnalisés. |
| `GET /rest/api/2/priority` | |
| `GET /rest/api/2/status` | |
| `GET /rest/api/2/issuetype` | |
| `GET /rest/api/2/resolution` | |

## Filtres

| Requête | Remarques |
|---|---|
| `GET /rest/api/2/filter/{id}` | Un filtre enregistré que l'utilisateur peut voir. |
| `GET /rest/api/2/filter/favourite` | Les filtres favoris de l'utilisateur. |
| `POST /rest/api/2/filter` | `name`, `jql`, `description`, `favourite`, `sharePermissions`. `name` et `jql` sont obligatoires. |
| `PUT /rest/api/2/filter/{id}` | Les mêmes champs. Seuls les champs donnés changent. Le propriétaire du filtre ou un administrateur. |
| `DELETE /rest/api/2/filter/{id}` | Le propriétaire du filtre ou un administrateur. 204. |

`sharePermissions` prend les types de Jira : `global`, `group`, `project`,
`projectRole` et `user`. `favourite` est propre à l'utilisateur de la requête.
Un administrateur qui retire le partage du filtre de quelqu'un d'autre ne le
voit plus, et reçoit 204 au lieu du filtre. Pour exécuter un filtre, cherchez
avec son `jql`.

## Agile : tableaux et sprints

Sous `/rest/agile/1.0` :

| Requête | Remarques |
|---|---|
| `GET /board` | Paramètre `projectKeyOrId`. |
| `GET /board/{id}` | |
| `GET /board/{id}/configuration` | Colonnes, filtre et estimation. |
| `GET /board/{id}/sprint` | Tableaux Scrum seulement. `state=active,future,closed`. |
| `GET /board/{id}/issue` | Les tickets du tableau, dans ses colonnes. |
| `GET /board/{id}/backlog` | |
| `POST /sprint` | `{"name": "Sprint 12", "originBoardId": 1}`, avec `goal`, `startDate` et `endDate` facultatifs. Crée un sprint futur sur un tableau Scrum ; répond 201 avec le sprint. |
| `GET /sprint/{id}` | |
| `POST /sprint/{id}`, `PUT /sprint/{id}` | Modifient le nom, l'objectif, les dates et l'état. Les deux fonctionnent en mise à jour partielle : ce qui n'est pas donné reste tel quel. `state: "active"` démarre un sprint futur ; `state: "closed"` termine un sprint actif, et ses tickets ouverts retournent au backlog. |
| `DELETE /sprint/{id}` | Seulement un sprint non démarré ; ses tickets retournent au backlog. |
| `GET /sprint/{id}/issue` | |
| `POST /sprint/{id}/issue` | `{"issues": ["DEMO-1"]}`. Vers un sprint ouvert. |
| `POST /backlog/issue` | `{"issues": ["DEMO-1"]}`. Hors de leur sprint ouvert. |
| `PUT /issue/rank`, `POST /issue/rank` | `{"issues": ["DEMO-1"], "rankBeforeIssue": "DEMO-2"}`, ou `rankAfterIssue`. L'un des deux, pas les deux. |
| `GET /epic/{key}` | |
| `GET /epic/{key}/issue` | |

Les tableaux ne peuvent être ni créés ni modifiés par l'API.

## Non pris en charge

Tout ce qui n'est pas listé plus haut n'est pas enregistré et répond 404.
Notamment :

- La version 3 de l'API de Jira, et les API de Jira Cloud.
- La création ou la modification d'utilisateurs, de groupes, de projets, de
  tableaux et de champs personnalisés.
- Le déplacement d'un ticket vers un autre projet, les opérations en masse,
  l'historique comme point d'accès à part, les propriétés de ticket, les
  tableaux de bord et les API de Jira Service Management.
- Les API des modules complémentaires.
