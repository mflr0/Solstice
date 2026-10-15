# Solstice

Suivi de tickets auto-hébergé, pensé pour reprendre une instance Jira Data
Center existante : Solstice lit directement la base de Jira (PostgreSQL ou
MySQL) et en importe tout l'essentiel, puis vit sa propre vie avec une
logique plus simple — pas de « schemes », une recherche qui se tape comme on
parle, des pages qui se mettent à jour d'elles-mêmes.

Backend en Go (un seul binaire, interface embarquée), base MariaDB,
interface en Svelte 5 + TypeScript. L'interface est en français ; le code
(identifiants, commentaires) est en anglais et toutes les chaînes affichées
sont dans `frontend/src/i18n/fr.ts`.

## Installation pour le développement

Prérequis : Go 1.27+, Node 22+, Docker (ou OrbStack).

```bash
docker compose up -d          # MariaDB pour Solstice, plus un Postgres et un MySQL « Jira » de test
corepack enable pnpm
(cd frontend && pnpm install)
```

Puis, dans deux terminaux :

```bash
cd backend && go run ./cmd/solstice serve      # API sur http://localhost:8080
cd frontend && pnpm dev                       # interface sur http://localhost:5173
```

Sans import, créez un premier compte :
`go run ./cmd/solstice user add -username moi -name "Moi" -admin` (le mot de
passe généré s'affiche).

## Importer un Jira

```bash
solstice import \
  -from postgres://jira:secret@db-jira:5432/jira \
  -tz Europe/Paris \
  -jira-home /var/atlassian/application-data/jira
```

- `-from` : la base de Jira, `postgres://…` ou `mysql://…`. Un accès en
  lecture suffit. Pour un résultat cohérent, arrêtez Jira (ou importez depuis
  une copie de la base).
- `-tz` : le fuseau horaire du serveur Jira (Jira › Administration › Infos
  système). MySQL stocke les dates sans fuseau, et les dates sans heure
  (échéances, versions) en dépendent sur les deux moteurs.
- `-jira-home` : facultatif ; permet de copier les fichiers des pièces
  jointes. Sans lui, seules leurs métadonnées sont reprises.
- `-replace` : écrase les données Solstice existantes (sinon l'import refuse
  une base non vide). L'import est rejouable : les identifiants Jira sont
  conservés.

Le rapport final liste ce qui a été repris et ce qui mérite un coup d'œil.
Ce que l'import reprend :

- **Comptes** : utilisateurs (annuaire prioritaire de Jira), groupes
  (groupes imbriqués aplatis), administrateurs. Les mots de passe des
  comptes internes restent valables (empreintes `{PKCS5S2}` de Jira, converties
  en Argon2id à la première connexion). Les comptes d'annuaires externes
  (LDAP…) n'ont pas de mot de passe : `solstice user passwd`. Les comptes sans
  accès applicatif dans Jira (hors groupes de licence) sont importés inactifs.
- **Projets** : catégories, responsables, anciennes clés des projets
  renommés, rôles et leurs membres, types de tickets proposés, composants,
  versions.
- **Droits, sans schemes** : chaque projet reçoit sa propre copie de son
  schéma de permissions et de son schéma de confidentialité. Ce qu'on ne sait
  pas traduire est retiré, jamais élargi : un ticket confidentiel ne devient
  jamais visible par plus de monde qu'avant.
- **Workflows** : chaque projet reçoit sa copie des workflows de son schéma
  (statuts et transitions ; le workflow système « jira », que Jira ne stocke
  pas en base, est recréé). Les conditions, validateurs et post-fonctions de
  Jira, qui sont du code Java, ne sont pas repris : Solstice applique ses
  propres règles (droits, résolution posée à l'arrivée dans un statut
  « terminé »).
- **Tickets** : champs, sous-tâches, liens, liens externes, étiquettes,
  observateurs, votes, commentaires (avec leurs restrictions de visibilité),
  temps saisis, pièces jointes, champs personnalisés, historique complet,
  anciennes clés des tickets déplacés.

Pas encore repris : boards et sprints (Jira Software), filtres enregistrés,
tableaux de bord, notifications. Voir la feuille de route.

## Production

```bash
(cd frontend && pnpm build)            # écrit dans backend/internal/web/dist
(cd backend && go build -o solstice ./cmd/solstice)
SOLSTICE_DB=mysql://solstice:…@db:3306/solstice SOLSTICE_DATA=/var/lib/solstice ./solstice serve
```

Le binaire contient l'interface. `SOLSTICE_DATA` reçoit les pièces jointes ;
`SOLSTICE_TZ` (par exemple `Europe/Paris`) est le fuseau des dates tapées dans
les recherches. Après un `solstice import -replace` sur une base en service,
le serveur reprend les nouveaux droits et catalogues d'eux-mêmes en moins
d'une minute.
Placez un reverse proxy HTTPS devant (le cookie de session passe en `Secure`
derrière un proxy qui envoie `X-Forwarded-Proto: https`).

## Tests

```bash
(cd backend && go test ./...)          # tests unitaires
(cd frontend && pnpm test && pnpm lint)
./scripts/e2e-import.sh                # bout en bout, avec les conteneurs de compose.yaml
```

`scripts/e2e-import.sh` génère la même base Jira synthétique sur PostgreSQL
et sur MySQL (`backend/cmd/jira-fixture`, à partir du `entitymodel.xml` de la
distribution Jira posée à côté du dépôt), importe les deux, vérifie que les
résultats sont identiques table par table et conformes à ce qui a été généré,
puis lance les tests d'API (droits, confidentialité, temps réel…) sur la base
importée.

## Organisation

```
backend/
  cmd/solstice/           le binaire : serve, migrate, import, user
  cmd/jira-fixture/     générateur de bases Jira synthétiques (tests)
  internal/api/         API HTTP JSON consommée par l'interface
  internal/importer/    import depuis Jira
  internal/jiradb/      connexion aux bases Jira (PostgreSQL, MySQL)
  internal/perm/        qui voit et fait quoi
  internal/search/      la recherche simple
  internal/jql/         le JQL
  internal/live/        notifications temps réel (SSE)
  internal/auth/        mots de passe et sessions
  internal/db/          connexion MariaDB et migrations
  internal/web/         sert l'interface compilée
frontend/
  src/core/             logique pure et testée (rendu du balisage wiki Jira…)
  src/ui/               composants Svelte
  src/lib/              état, routeur, client API
  src/i18n/fr.ts        tous les textes affichés
scripts/e2e-import.sh   test de bout en bout de l'import
```

## Différences assumées avec Jira

- **Pas de schemes.** Permissions, confidentialité et types de tickets se
  règlent dans le projet, sur une seule page. Modifier un projet n'en touche
  jamais un autre.
- **Moins de permissions, plus larges.** Résoudre et fermer font partie de
  « changer le statut » ; chacun peut modifier ou supprimer ses propres
  commentaires.
- **Les administrateurs configurent, ils ne lisent pas tout.** Comme dans
  Jira, être administrateur ne donne pas accès aux projets ni aux tickets
  confidentiels.
- **Recherche simple, et JQL** : `panier @marie est:ouvert #urgent tri:priorité`
  dans la même barre que `project = DEMO AND assignee = currentUser() ORDER BY
  priority DESC` ; Solstice reconnaît la syntaxe tout seul. Le JQL couvre les
  champs standards, les champs personnalisés (par nom ou `cf[id]`), les dates
  relatives (`-7d`, `startOfWeek()`), `currentUser()` et `membersOf()`. Pas
  encore : les opérateurs d'historique (`WAS`, `CHANGED`) et les sprints.
- **Les sous-tâches suivent la confidentialité de leur parent**, toujours.
- **Connexions** : après cinq échecs pour un même identifiant ou une même
  adresse, les tentatives sont suspendues une minute, puis davantage.

## Feuille de route

1. **Socle** (en cours) : import, tickets, commentaires, pièces jointes,
   recherche simple, droits par projet, temps réel.
2. **Workflows et tableaux** (commencé : workflows importés et appliqués, JQL) :
   éditeur de workflows par projet, boards Kanban et Scrum, sprints, filtres
   enregistrés.
3. **Administration et intégrations** : édition des champs personnalisés,
   notifications par e-mail, tableaux de bord, API REST compatible Jira pour
   les scripts existants.

## Note sur Jira

La distribution Jira posée à côté du dépôt sert uniquement de référence pour
le schéma de sa base (`entitymodel.xml`), afin de lire les données de Jira.
Elle n'est pas versionnée et aucun code, style ou visuel d'Atlassian n'est
repris dans Solstice.

## Licence

Solstice est un logiciel libre, sous licence GNU Affero General Public
License version 3 ou (à votre choix) toute version ultérieure
(AGPL-3.0-or-later) : voir [LICENSE](LICENSE).
