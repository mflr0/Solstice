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

**`SOLSTICE_DB` est obligatoire.** Solstice n'a pas de base par défaut et
n'utilise pas d'office celle de `compose.yaml`. Sans `SOLSTICE_DB`,
`migrate`, `user` et `import` s'arrêtent sur « no database configured », et
`serve` ouvre l'assistant d'installation au lieu de la base de développement.
Exportez la variable dans chaque terminal, ou écrivez-la une fois pour toutes
dans `backend/solstice.conf` (lu dans le dossier courant, donc en lançant
depuis `backend/`) :

```
SOLSTICE_DB=mysql://solstice:solstice@localhost:13306/solstice
```

Puis, dans deux terminaux :

```bash
export SOLSTICE_DB=mysql://solstice:solstice@localhost:13306/solstice   # inutile avec backend/solstice.conf
cd backend && go run ./cmd/solstice serve      # API sur http://localhost:8080
cd frontend && pnpm dev                       # interface sur http://localhost:5173
```

Sur une base sans compte, le serveur ouvre l'assistant d'installation
(`/setup`, code affiché dans son journal). Sinon, créez un premier compte :
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
SOLSTICE_DATA=/var/lib/solstice ./solstice serve
```

Au premier démarrage, sans base configurée ou sur une base vide, Solstice
n'ouvre que son assistant d'installation sur `/setup` : il affiche dans son
journal un code à usage unique (aussi écrit dans `<données>/setup-code`), puis
demande la base, l'adresse publique, le fuseau horaire, et crée
l'administrateur ou importe un Jira. Les réglages vont dans un fichier de
lignes `CLÉ=valeur` (`-config`, `SOLSTICE_CONFIG`, sinon `./solstice.conf`,
sinon `/etc/solstice/solstice.conf`), que les variables d'environnement
surchargent : `solstice -h` les liste. `GET /healthz` répond 200 quand la
base répond, 503 sinon.

Le binaire contient l'interface. `SOLSTICE_DATA` reçoit les pièces jointes ;
`SOLSTICE_TZ` (par exemple `Europe/Paris`) est le fuseau des dates tapées dans
les recherches. Après un `solstice import -replace` sur une base en service,
le serveur reprend les nouveaux droits et catalogues d'eux-mêmes en moins
d'une minute.
Placez un reverse proxy HTTPS devant (le cookie de session passe en `Secure`
derrière un proxy qui envoie `X-Forwarded-Proto: https`).

## Courriels et webhooks

- **Serveur SMTP** : il se règle dans l'administration (hôte, port, sécurité
  aucune / STARTTLS / TLS, identifiant, expéditeur), avec un envoi d'essai qui
  affiche l'erreur exacte du serveur. `SOLSTICE_BASE_URL` (l'adresse publique)
  sert aux liens des courriels ; sans elle, ils partent sans liens.
- **Secrets** : le mot de passe SMTP et les secrets des webhooks sont chiffrés
  (AES-256-GCM) avec la clé `SOLSTICE_SECRET_KEY` (32 octets en base64 ou en
  hexadécimal, par exemple `openssl rand -base64 32`), ou à défaut celle de
  `<données>/secret.key`, créée au premier besoin. Sauvegardez-la avec la
  base : sans elle, les secrets sont à ressaisir.
- **Notifications** : observateurs, rapporteur, assigné et personnes
  mentionnées (`[~identifiant]`). Chacun choisit ses événements, l'envoi
  immédiat (les changements d'un même ticket à moins de deux minutes
  d'intervalle partent dans un seul courriel) ou un résumé quotidien, et peut
  demander un rappel matinal de ses échéances. Un projet peut couper ses
  notifications ou ajouter un groupe ou un rôle aux destinataires. Les droits
  sont vérifiés à l'écriture de chaque courriel : personne ne reçoit un ticket
  ou un commentaire restreint qu'il ne peut pas voir.
- **Abonnements aux filtres** : les résultats d'un filtre enregistré, chaque
  jour ou certains jours de la semaine, tels que chaque destinataire les voit
  (200 tickets au plus, avec un lien vers la suite).
- **Webhooks** (administration) : événements et contenu au format des
  webhooks de Jira Server (`jira:issue_updated`, `comment_created`,
  `sprint_started`…), filtre JQL facultatif, signature
  `X-Hub-Signature: sha256=…` (HMAC-SHA256 du corps) quand un secret est
  défini, nouvelles tentatives espacées, journal des 100 derniers envois.
  Les destinations peuvent être internes (réseau privé, machine locale) : les
  intégrations auto-hébergées le sont souvent. Les plages lien-local
  (169.254.0.0/16, fe80::/10) et les adresses de métadonnées des clouds
  (169.254.169.254, fd00:ec2::254…) sont en revanche toujours refusées,
  vérifiées après résolution DNS à chaque connexion ; les redirections ne
  sont pas suivies et aucun proxy n'est utilisé. Comme dans Jira, un webhook
  voit tous les tickets, confidentiels compris : restreignez-le par son
  filtre JQL.

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
  internal/events/      bus d'événements : temps réel, courriels, webhooks
  internal/notify/      notifications par courriel, résumés, abonnements, rappels
  internal/mail/        réglages SMTP, envoi, file d'attente des courriels
  internal/webhook/     webhooks au format Jira, garde contre les SSRF
  internal/secret/      chiffrement des secrets enregistrés
  internal/settings/    réglages de l'instance en base
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
