# Solstice

[English](README.md)

Solstice est un outil de suivi de tickets auto-hébergé. Chaque projet porte
ses propres droits et workflows, les tableaux couvrent Kanban et Scrum, la
recherche se fait par filtres ou en JQL, et les pages se mettent à jour
d'elles-mêmes quand quelqu'un d'autre modifie un ticket. Il peut reprendre une
instance Jira Data Center ou Server en lisant directement sa base, historique
compris.

Un seul binaire Go sert l'API et l'interface web embarquée (Svelte 5 et
TypeScript). Les données sont dans MariaDB 10.6+ ou MySQL 8.0+.

## Fonctionnalités

- Tickets : sous-tâches, liens, pièces jointes, commentaires à visibilité
  restreinte, temps passé, observateurs, votes, champs personnalisés et
  historique complet. L'éditeur de texte riche enregistre descriptions et
  commentaires en balisage wiki, sans le modifier.
- Projets sans schémas partagés : droits, confidentialité et workflows
  appartiennent à chaque projet. L'éditeur de workflows montre les statuts,
  une grille des transitions et un schéma en direct.
- Tableaux : Kanban et Scrum, colonnes avec limites WIP, couloirs, filtres
  rapides, backlog et sprints. Les cartes se déplacent à la souris ou au
  clavier.
- Recherche : une barre de filtres avec le nombre de tickets par valeur, la
  recherche par personne et par rôle, un éditeur JQL avec complétion (y
  compris `WAS` et `CHANGED`), des filtres enregistrés avec abonnements par
  e-mail, l'export CSV.
- Notifications par e-mail, envoyées tout de suite (les changements d'un même
  ticket sont regroupés) ou en résumé quotidien, avec les mentions et des
  rappels d'échéance. Webhooks.
- Intégrations : une API REST compatible avec `/rest/api/2` et
  `/rest/agile/1.0` de Jira, pour que les scripts et outils existants
  continuent de fonctionner ; des jetons d'accès personnels ; un journal
  d'audit.
- Interface : huit thèmes en clair et en sombre, en français, anglais,
  allemand, italien et hindi, utilisable sur téléphone.

## Installation

### Avec Docker (recommandé)

```bash
git clone https://github.com/mflr0/Solstice.git
cd Solstice/deploy/docker
./install.sh
```

Le script écrit des mots de passe aléatoires pour la base dans `.env`,
démarre MariaDB et Solstice, puis affiche l'adresse de l'assistant
d'installation et son code à usage unique. Le relancer garde les données et
redémarre sur les sources du moment.

### Sur un serveur Linux (systemd)

À partir d'une archive publiée :

```bash
tar xzf solstice-<version>-linux-amd64.tar.gz
cd solstice-<version>-linux-amd64
sudo ./install.sh --with-mariadb
```

Le script crée l'utilisateur système `solstice`, `/etc/solstice` et
`/var/lib/solstice`, ainsi qu'un service systemd durci. `--with-mariadb`
installe aussi MariaDB et crée la base et son compte ; sans cette option,
l'assistant demande une base existante. Relancer le script avec une archive
plus récente met à jour sur place.

### Premier démarrage : l'assistant d'installation

Tant qu'aucune base n'est configurée, ou que la base n'a encore aucun compte,
Solstice ne sert que son assistant d'installation sur `/setup`. Il affiche un
code à usage unique dans son journal (aussi écrit dans `<données>/setup-code`).
L'assistant demande la base si elle n'est pas déjà fixée, puis l'adresse
publique et le fuseau horaire, et enfin crée l'administrateur ou importe une
instance Jira.

### Configuration

Les réglages sont dans un fichier de lignes `CLÉ=valeur`, trouvé par
`-config`, puis `SOLSTICE_CONFIG`, puis `./solstice.conf`, puis
`/etc/solstice/solstice.conf`. Les variables d'environnement du même nom
l'emportent ; `solstice -h` les liste. `GET /healthz` répond 200 quand la base
répond, 503 sinon.

Placez un reverse proxy HTTPS devant Solstice. Le cookie de session passe en
`Secure` derrière un proxy qui envoie `X-Forwarded-Proto: https`.

Le mot de passe SMTP et les secrets des webhooks sont chiffrés avec la clé
`SOLSTICE_SECRET_KEY` (32 octets en base64 ou en hexadécimal, par exemple
`openssl rand -base64 32`), ou à défaut avec `<données>/secret.key`, créée au
premier besoin. Sauvegardez cette clé avec la base et le dossier de données :
sans elle, ces secrets sont à ressaisir.

## Migrer depuis Jira

L'assistant d'installation sait importer une instance Jira, la ligne de
commande aussi :

```bash
solstice import \
  -from postgres://jira:secret@db-jira:5432/jira \
  -tz Europe/Paris \
  -jira-home /var/atlassian/application-data/jira
```

- `-from` : la base de Jira, `postgres://…` ou `mysql://…`. Un accès en
  lecture suffit. Pour un résultat cohérent, arrêtez Jira ou importez depuis
  une copie de la base.
- `-tz` : le fuseau horaire du serveur Jira (Administration › Infos système).
  MySQL stocke les dates sans fuseau, et les dates sans heure (échéances,
  versions) en dépendent avec les deux moteurs.
- `-jira-home` : facultatif ; copie les fichiers des pièces jointes. Sans lui,
  seules leurs métadonnées sont reprises.
- `-replace` : écrase les données Solstice existantes (sinon l'import refuse
  une base non vide). L'import peut être rejoué : les identifiants sont
  conservés.

Le rapport final liste ce qui a été repris et ce qui mérite un coup d'œil. Ce
qui est repris :

- Comptes : utilisateurs (annuaire prioritaire), groupes (groupes imbriqués
  aplatis), administrateurs. Les comptes internes gardent leur mot de passe :
  les empreintes `{PKCS5S2}` sont converties en Argon2id à la première
  connexion. Les comptes d'un annuaire externe (LDAP…) n'ont pas de mot de
  passe : définissez-en un avec `solstice user passwd`. Les comptes sans accès
  applicatif sont importés inactifs.
- Projets : catégories, responsables, anciennes clés des projets renommés,
  rôles et leurs membres, types de tickets proposés, composants, versions.
- Droits : chaque projet reçoit sa propre copie de son schéma de permissions
  et de son schéma de confidentialité. Ce qui ne se traduit pas est retiré,
  jamais élargi : un ticket confidentiel ne devient jamais visible par plus de
  monde qu'avant.
- Workflows : chaque projet reçoit sa copie des workflows de son schéma
  (statuts et transitions ; le workflow système, qui n'est pas stocké en
  base, est recréé). Les conditions, validateurs et post-fonctions sont du
  code Java et ne sont pas repris : Solstice applique ses propres règles
  (droits, résolution posée à l'arrivée dans un statut terminé).
- Tickets : champs, sous-tâches, liens, liens externes, étiquettes,
  observateurs, votes, commentaires avec leurs restrictions de visibilité,
  temps saisis, pièces jointes, champs personnalisés, historique complet,
  anciennes clés des tickets déplacés.
- Tableaux et sprints, avec l'ordre des cartes (rang) et les épopées ; filtres
  enregistrés avec leurs partages et favoris.

Non repris : les tableaux de bord, et les schémas de notification (Solstice a
ses propres réglages de notifications).

## E-mails et webhooks

- Serveur SMTP : il se règle dans l'administration (hôte, port, sécurité
  aucune / STARTTLS / TLS, identifiant, expéditeur), avec un envoi d'essai qui
  affiche l'erreur exacte du serveur. L'adresse publique
  (`SOLSTICE_BASE_URL`) sert aux liens des e-mails ; sans elle, ils partent
  sans liens.
- Notifications : observateurs, rapporteur, assigné et personnes mentionnées
  (`[~identifiant]`). Chacun choisit ses événements, l'envoi immédiat (les
  changements d'un même ticket à moins de deux minutes d'intervalle partent
  dans un seul e-mail) ou un résumé quotidien, et peut demander un rappel
  matinal de ses échéances. Un projet peut couper ses notifications ou ajouter
  un groupe ou un rôle aux destinataires. Les droits sont vérifiés à
  l'écriture de chaque e-mail : personne ne reçoit un ticket ou un commentaire
  restreint qu'il ne peut pas voir.
- Abonnements aux filtres : les résultats d'un filtre enregistré, chaque jour
  ou certains jours de la semaine, tels que chaque destinataire les voit
  (200 tickets au plus, avec un lien vers la suite).
- Webhooks (administration) : événements et contenu au format des webhooks
  de Jira Server (`jira:issue_updated`, `comment_created`,
  `sprint_started`…), filtre JQL facultatif, signature
  `X-Hub-Signature: sha256=…` (HMAC-SHA256 du corps) quand un secret est
  défini, nouvelles tentatives espacées, journal des 100 derniers envois. Les
  destinations peuvent être internes (réseau privé, machine locale), comme
  souvent pour les intégrations auto-hébergées. Les plages lien-local
  (169.254.0.0/16, fe80::/10) et les adresses de métadonnées des clouds
  (169.254.169.254, fd00:ec2::254…) sont toujours refusées, vérifiées après
  résolution DNS à chaque connexion ; les redirections ne sont pas suivies et
  aucun proxy n'est utilisé. Un webhook voit tous les tickets, confidentiels
  compris : restreignez-le par son filtre JQL.

## Développement

Prérequis : Go 1.27+, Node 22+ avec pnpm, Docker (ou OrbStack).

```bash
docker compose up -d          # MariaDB pour Solstice, plus un PostgreSQL et un MySQL « Jira » de test
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
(`/setup`, code dans son journal). Sinon, créez un premier compte :
`go run ./cmd/solstice user add -username moi -name "Moi" -admin` (le mot de
passe généré s'affiche).

`make build` produit le binaire unique avec l'interface embarquée,
`make docker` l'image, et `make release VERSION=x.y.z` les archives pour
Linux, macOS et Windows avec leurs SHA256SUMS.

### Tests

```bash
(cd backend && go test ./...)          # tests unitaires
(cd frontend && pnpm test && pnpm lint)
./scripts/e2e-import.sh                # bout en bout, avec les conteneurs de compose.yaml
```

`scripts/e2e-import.sh` génère la même base Jira synthétique sur PostgreSQL et
sur MySQL (`backend/cmd/jira-fixture`, à partir du `entitymodel.xml` d'une
distribution Jira posée à côté du dépôt), importe les deux, vérifie que les
résultats sont identiques table par table et conformes à ce qui a été généré,
puis lance les tests d'API (droits, confidentialité, temps réel…) sur la base
importée.

### Organisation

```
backend/
  cmd/solstice/         le binaire : serve, migrate, import, user, version, health
  cmd/jira-fixture/     générateur de bases Jira synthétiques (tests)
  internal/api/         API HTTP JSON de l'interface, couche de compatibilité REST
  internal/importer/    import depuis Jira
  internal/jiradb/      connexion aux bases Jira (PostgreSQL, MySQL)
  internal/perm/        qui voit et fait quoi
  internal/search/      la recherche simple
  internal/jql/         le JQL
  internal/live/        mises à jour en temps réel (SSE)
  internal/events/      bus d'événements : temps réel, e-mails, webhooks
  internal/notify/      notifications par e-mail, résumés, abonnements, rappels
  internal/mail/        réglages SMTP, envoi, file d'attente des e-mails
  internal/webhook/     webhooks, garde contre les SSRF
  internal/secret/      chiffrement des secrets enregistrés
  internal/settings/    réglages de l'instance en base
  internal/config/      le fichier de configuration
  internal/audit/       journal d'audit
  internal/i18n/        messages du serveur en cinq langues
  internal/auth/        mots de passe et sessions
  internal/db/          connexion MariaDB et migrations
  internal/web/         sert l'interface compilée
frontend/
  src/core/             logique pure et testée (rendu du balisage wiki, JQL…)
  src/ui/               composants Svelte
  src/lib/              état, routeur, client API
  src/i18n/             textes de l'interface, un fichier par langue
  src/themes/           les thèmes et leur contrat (ANATOMY.md)
deploy/                 installation Docker et Linux
scripts/                test d'import de bout en bout, archives de publication
```

## Choix de conception

- Pas de schémas. Droits, confidentialité et types de tickets se règlent dans
  le projet, sur une seule page. Modifier un projet n'en touche jamais un
  autre.
- Moins de permissions, plus larges. Résoudre et fermer font partie de
  « changer le statut » ; chacun peut modifier ou supprimer ses propres
  commentaires.
- Les administrateurs configurent, ils ne lisent pas tout : être
  administrateur ne donne accès ni aux projets ni aux tickets confidentiels.
- Les sous-tâches suivent toujours la confidentialité de leur parent.
- Connexion : après cinq échecs pour un même identifiant ou une même adresse,
  les tentatives sont suspendues une minute, puis davantage.

## Feuille de route

- 1.0, en cours : une administration pour tout régler depuis l'interface
  (projets, composants, versions, champs, groupes, réglages de l'instance),
  et les mises à jour appliquées depuis l'administration.
- 1.1 : tableaux de bord et rapports agiles, modification en masse et
  raccourcis clavier, annuaire LDAP/Active Directory et authentification
  unique.

## Marques

Jira et Atlassian sont des marques d'Atlassian. Solstice est un projet
indépendant, sans lien avec Atlassian ni approuvé par elle. Pour lire les
bases Jira, l'import s'appuie sur la description du schéma
(`entitymodel.xml`) d'une distribution Jira posée à côté du dépôt pendant le
développement ; elle n'est pas versionnée, et aucun code, style ou visuel
d'Atlassian n'est utilisé dans Solstice.
