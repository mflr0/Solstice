# Migrer depuis Jira

Solstice peut reprendre une instance Jira Server ou Data Center. Il lit
directement la base de données de Jira, donc un accès en lecture suffit, et il
n'y écrit jamais. Vous pouvez importer autant de fois que vous voulez, essayer
sur une copie, et revenir à Jira si vous changez d'avis.

Cette page couvre ce que demande l'import, comment le lancer, comment préparer
une migration, et ce qui est repris ou non. L'installation de Solstice est
dans [install.md](install.md), les sauvegardes dans [backups.md](backups.md).

## Avant de commencer

**Sources prises en charge.**

- Jira Server ou Data Center, version 7 ou plus récente. Solstice vérifie le
  schéma, pas un numéro de version : une base sans la colonne
  `schemepermissions.permission_key` est refusée avec un message qui demande de
  mettre Jira à jour d'abord.
- Une base Jira sous PostgreSQL (`postgres://…`) ou MySQL (`mysql://…`). Les
  autres moteurs ne sont pas pris en charge comme source.
- Solstice lui-même fonctionne sur MariaDB.

**Ce qu'il vous faut.**

- L'adresse, le nom, l'utilisateur et le mot de passe de la base Jira. Un
  utilisateur qui peut la lire suffit.
- Le fuseau horaire du serveur Jira (Jira, Administration, Informations
  système). MySQL stocke les dates sans fuseau horaire, et les dates sans heure
  (échéances, versions) en dépendent avec les deux moteurs. Une valeur fausse
  décale ces dates.
- Facultatif : le répertoire personnel de Jira, lisible depuis la machine où
  tourne Solstice, pour copier les fichiers des pièces jointes. Sans lui, seules
  les métadonnées des pièces jointes sont importées.

**Pour un résultat cohérent**, arrêtez d'abord Jira, ou importez depuis une
copie de sa base. Importer depuis une base dans laquelle on écrit encore peut
donner un résultat incohérent.

**L'import remplace tout ce que contient Solstice.** Il n'y a pas de fusion.
Avant tout import sur une instance qui contient des données, sauvegardez sa
base et son répertoire de données : c'est le seul moyen de revenir en arrière.
Voir [backups.md](backups.md).

## Lancer l'import

Il y a trois façons. Elles lancent le même import.

### La ligne de commande

```sh
solstice import \
  -from postgres://jira:secret@db-jira.example.org:5432/jira \
  -tz Europe/Paris \
  -jira-home /var/atlassian/application-data/jira
```

| Option | Signification |
|---|---|
| `-from` | La base Jira : `postgres://utilisateur:mot_de_passe@hôte/base` ou `mysql://utilisateur:mot_de_passe@hôte/base`. Obligatoire. |
| `-tz` | Le fuseau horaire du serveur Jira, par exemple `Europe/Paris`. Obligatoire. |
| `-jira-home` | Le répertoire personnel de Jira, pour copier les fichiers des pièces jointes. Facultatif. |
| `-replace` | Efface les données de Solstice avant d'importer. Sans cette option, l'import refuse de s'exécuter sur une base qui contient déjà des utilisateurs ou des projets. |
| `-json` | Affiche le rapport en JSON plutôt qu'en texte. |

La commande lit la même configuration que le serveur (`SOLSTICE_DB_URL`,
`SOLSTICE_DATA`…) et applique d'abord les migrations de base en attente. Les
fichiers des pièces jointes sont copiés dans `<data>/attachments`.

Encodez en pourcentage les caractères spéciaux des mots de passe dans les URL
(`@` devient `%40`). Les seules options acceptées après `?` sont `tls`
(`true`, `false`, `skip-verify`, `preferred`), `timeout`, `readTimeout` et
`charset` pour MySQL, et `sslmode` et `connect_timeout` pour PostgreSQL. Toute
autre option est refusée.

Le rapport indique, pour chaque type d'enregistrement, combien ont été importés
et combien ont été ignorés (références orphelines), puis une liste « Check » de
points qui méritent un coup d'œil. Lisez-le.

### L'assistant d'installation

Sur une nouvelle installation, l'assistant d'installation de `/setup` peut
importer une instance Jira au lieu de créer un administrateur. Vous saisissez
l'adresse de la base Jira, le fuseau horaire et, facultativement, le
répertoire personnel de Jira. Un bouton de test se connecte à la base et
affiche le nombre de projets, de tickets et d'utilisateurs trouvés, et si le
répertoire de Jira contient des pièces jointes. Puis l'import s'exécute et
montre sa progression. Ensuite, l'assistant demande qui administre Solstice :
vous choisissez un compte importé qui peut se connecter (les administrateurs de
Jira viennent en premier), ou vous créez un nouvel administrateur.

Le répertoire personnel de Jira doit être un chemin absolu qui existe sur la
machine où tourne Solstice.

### La page d'administration

Un administrateur peut importer depuis Administration, Import Jira, sur une
instance en service. La page demande les mêmes trois valeurs que l'assistant et
propose un bouton de test. Pour démarrer, vous devez cocher que vous avez une
sauvegarde et saisir le nom de la base de Solstice.

Pendant l'import, Solstice est en maintenance. Sous `/api/` et `/rest/`, seules
trois requêtes répondent encore : `GET /api/settings/public` et, pour
l'administrateur qui a lancé l'import, `GET /api/admin/import` (la progression)
et `GET /api/me`. Toutes les autres requêtes répondent 503. Gardez la page de
progression ouverte. À la fin, rechargez.

Un seul import s'exécute à la fois par base, qu'il soit lancé d'ici, depuis
l'assistant ou depuis la ligne de commande.

Ce qu'un import conserve de la configuration propre à Solstice, depuis
l'administration comme avec `-replace` :

- Conservés tels quels : les réglages de l'instance (généraux, e-mail y
  compris le mot de passe SMTP, rétention de l'audit), les webhooks (leur
  journal d'envois est vidé) et le journal d'audit.
- Conservés pour les comptes qui existent après l'import, retrouvés par leur
  identifiant : jetons d'accès personnels, préférences de notification, langue
  de l'interface, dernière connexion, sessions ouvertes et droits
  d'administrateur.
- Les comptes créés dans Solstice dont l'identifiant n'existe pas dans Jira
  sont conservés, avec leur mot de passe, mais sans groupes ni rôles.
- L'administrateur qui lance l'import depuis l'administration garde son compte,
  son mot de passe, ses droits et sa session, même si Jira a un compte du même
  nom.
- Remplacés : tout ce qui vient de Jira, c'est-à-dire les utilisateurs, les
  groupes, les projets, les tickets et leur contenu, les catalogues, les champs
  personnalisés, les filtres, les tableaux et les sprints. Les abonnements aux
  filtres, les réglages de notification des projets, les e-mails en attente et
  les envois de webhooks disparaissent avec eux.

## Ce qui est importé

Les identifiants propres à Jira sont conservés (clés et numéros de tickets,
identifiants de tableaux, de sprints, de filtres et de pièces jointes), si bien
que les liens entre enregistrements survivent et qu'un import peut être
relancé.

| Domaine | Ce qui est repris |
|---|---|
| Utilisateurs | Un compte par utilisateur Jira, depuis l'annuaire de plus haute priorité. Nom affiché, adresse e-mail, actif ou non. Les utilisateurs supprimés de Jira mais encore cités dans des tickets sont conservés comme comptes inactifs. |
| Mots de passe | Les mots de passe de l'annuaire interne sont conservés sous forme de hachages Jira (`{PKCS5S2}`) et convertis dans le format propre à Solstice à la première connexion de la personne. Chacun garde son mot de passe. |
| Groupes | Les groupes et leurs membres. Les groupes imbriqués sont aplatis : une personne appartient aux groupes du groupe dans lequel elle se trouve. |
| Administrateurs | Les membres des groupes qui détiennent la permission Administer ou System Administer de Jira deviennent administrateurs de Solstice. |
| Accès à l'application | Les comptes qui ne sont dans aucun groupe sous licence (et ne sont pas administrateurs) sont importés inactifs, comme Jira les aurait refusés. |
| Projets | Clé, nom, description, responsable, catégorie, anciennes clés des projets renommés, types de ticket proposés par chaque projet, composants, versions. |
| Rôles | Les rôles de projet et qui les tient dans chaque projet. |
| Permissions | Chaque projet reçoit sa propre copie de son schéma de permissions et de son schéma de sécurité des tickets (niveaux et membres). |
| Catalogues | Types de ticket, statuts et leurs catégories, priorités, résolutions, types de lien. |
| Workflows | Chaque projet reçoit sa propre copie des workflows que son schéma utilise : statuts, transitions, statut initial, et quels types de ticket suivent quel workflow. |
| Champs personnalisés | Définitions, options (niveaux en cascade compris), projets et types de ticket concernés, et toutes leurs valeurs. |
| Tickets | Tous les champs, sous-tâches, épopées, liens, liens distants, étiquettes, observateurs, votes, niveaux de sécurité, anciennes clés des tickets déplacés. |
| Commentaires et temps saisi | Avec leurs restrictions de visibilité à des groupes et des rôles. |
| Pièces jointes | Les métadonnées toujours. Les fichiers quand `-jira-home` est donné. |
| Historique | L'historique complet des modifications de chaque ticket. |
| Filtres | Les filtres enregistrés avec leur JQL, leurs partages et leurs favoris. |
| Jira Software | Les tableaux avec leurs colonnes, filtres rapides et couloirs, les sprints et les tickets qu'ils ont contenus, le classement dans les tableaux, les liens d'épopée. |

À savoir :

- **Les permissions ne sont jamais élargies.** Ce que Solstice ne sait pas
  exprimer (permissions accordées par un champ personnalisé ou à un portail de
  service desk, par exemple) est retiré, et le rapport dit combien. Un ticket
  confidentiel ne devient jamais visible de plus de monde qu'avant. Il en va de
  même des partages de filtres.
- **Le comportement des workflows n'est pas repris.** Les conditions,
  validateurs et post-fonctions de Jira sont du code Java. Solstice applique
  ses propres règles : les permissions décident de qui peut faire une
  transition, et un ticket qui atteint un statut terminé sans résolution reçoit
  la résolution par défaut.
- **Un projet sans schéma de permissions** est importé sans accès pour
  personne. Le rapport le nomme. Corrigez-le dans ses réglages.

## Ce qui n'est pas importé

- Les tableaux de bord et leurs gadgets.
- Les schémas de notification. Solstice a ses propres réglages de
  notification : chacun choisit ses événements, et chaque projet peut ajouter
  des destinataires.
- Les écrans, les configurations de champs et les autres schémas qui décident
  quel champ est affiché ou obligatoire où. Solstice affiche les champs qui
  s'appliquent au projet et au type de ticket.
- Les données de Jira Service Management et celles des autres modules
  complémentaires. Seuls les tableaux, sprints, classements et épopées de Jira
  Software sont lus.
- Les mots de passe des comptes d'un annuaire externe (LDAP et similaires), ou
  stockés dans un ancien format. Ces comptes sont importés sans mot de passe.
  Le rapport compte les comptes actifs concernés. Activez la connexion LDAP
  (voir [l'administration](administration.md#ldap-et-active-directory)),
  définissez-en un avec `solstice user passwd -username NOM`, ou envoyez un
  lien de réinitialisation depuis l'administration.
- Les fichiers des pièces jointes, quand `-jira-home` n'est pas donné. Le
  rapport le dit. Relancez l'import avec `-jira-home`, ou acceptez que ces
  fichiers manquent : leurs entrées répondent 404 au téléchargement.
- La configuration du serveur Jira : serveur de messagerie, liens
  d'applications, licence. Réglez le serveur de messagerie dans
  Administration, E-mail.

Les champs personnalisés dont Solstice ne sait pas modifier le type Jira sont
importés et affichés, mais non modifiables (« type Jira, affichage seul »).

## Préparer la migration

### 1. Répétition sur une copie

Ne commencez pas par votre Jira de production. Restaurez une copie de sa base
sur un serveur de base de données à part, installez Solstice sur une machine de
test (voir [install.md](install.md)), et importez depuis la copie :

```sh
solstice import -from postgres://jira:secret@db-copy.example.org:5432/jira \
  -tz Europe/Paris -jira-home /mnt/jira-home-copy -replace
```

Vérifiez le rapport. Ouvrez des projets, des tickets à longue histoire, des
tableaux et des sprints, des commentaires restreints, et quelques tickets
confidentiels en tant que personne qui ne doit pas les voir. Vérifiez que les
gens peuvent se connecter. Chronométrez l'import, pour savoir combien de temps
demande le basculement.

Vous pouvez recommencer autant que nécessaire. Chaque exécution avec
`-replace` repart des données de Jira.

### 2. Exécution en parallèle

Pendant que les équipes continuent de travailler dans Jira, gardez votre
instance de test et laissez-la essayer à quelques personnes. Il n'y a pas
d'import incrémental : chaque import remplace tout, donc ce qui est saisi dans
l'instance de test est perdu au prochain import, sauf ce que Solstice conserve
(voir plus haut). Prévenez les testeurs.

Profitez de ce temps pour régler ce que Solstice ne reprend pas : configurer le
serveur de messagerie, revoir les permissions des projets importants, définir
l'adresse publique et décider qui est administrateur.

### 3. Le jour du basculement

1. Annoncez le gel. Arrêtez Jira, ou au moins empêchez d'y écrire, pour que la
   base ne bouge plus.
2. Sauvegardez la base de Jira et, si vous copiez les pièces jointes, assurez-
   vous que son répertoire personnel est complet. N'y touchez plus.
3. Sur le Solstice de production, sauvegardez sa base et son répertoire de
   données ([backups.md](backups.md)).
4. Lancez l'import avec `-replace` et `-jira-home`, depuis la ligne de commande
   ou la page d'administration.
5. Lisez le rapport. Traitez la liste « Check » : comptes sans mot de passe,
   projets sans accès, fichiers de pièces jointes manquants.
6. Vérifiez quelques projets, tableaux et tickets, et connectez-vous avec un
   compte ordinaire.
7. Faites pointer votre nom de domaine vers Solstice (section suivante) et
   annoncez le changement.

### Garder le même nom de domaine

Les pages de tickets de Solstice utilisent la forme d'adresse de Jira.
Solstice sert `/browse/KEY-1`, donc les liens des favoris, des wikis, des
messageries et des anciens e-mails continuent de fonctionner quand le nom de
domaine pointe vers Solstice. Une ancienne clé de projet est aussi résolue :
si un projet a été renommé dans Jira, `/browse/ANCIENNE-12` retrouve toujours
le ticket. Autres adresses qui continuent de fonctionner :

- `/projects/KEY` ouvre le projet.
- `/secure/RapidBoard.jspa?rapidView=ID` ouvre le tableau portant cet
  identifiant (avec `view=planning`, son backlog), et
  `/secure/RapidView.jspa?rapidView=ID` ouvre ses réglages. L'import conserve
  les identifiants des tableaux.
- `/rest/api/2/…` et `/rest/agile/1.0/…` : voir [rest-api.md](rest-api.md).
- `/secure/attachment/{id}/{filename}`, l'adresse des fichiers des pièces
  jointes.

Les autres pages de Jira ne sont pas servies, comme les tableaux de bord et les
pages d'administration (`/secure/…` hormis les deux tableaux ci-dessus).
Solstice répond par sa propre page « introuvable ».

Pour garder le nom de domaine :

1. Réglez l'adresse publique de Solstice sur ce nom de domaine
   ([administration.md](administration.md)), et placez-le derrière HTTPS
   ([reverse-proxy.md](reverse-proxy.md)).
2. Le jour du basculement, une fois l'import vérifié, changez l'enregistrement
   DNS ou le proxy pour que le nom mène à Solstice. Si Jira se trouvait sous un
   chemin comme `/jira`, redirigez ce chemin.
3. Baissez la durée de vie (TTL) du DNS un jour ou deux avant, pour que le
   changement se propage vite.

Les scripts qui appellent l'API REST avec un jeton ont besoin d'un nouveau
jeton Solstice : les jetons ne sont pas importés. Voir
[rest-api.md](rest-api.md).

### Retour en arrière

Solstice n'écrit jamais dans la base de Jira. Tant que vous laissez intactes la
base et le répertoire personnel de Jira, revenir en arrière consiste à refaire
pointer le nom de domaine vers Jira et à le démarrer.

Ce qu'on ne peut pas ramener, c'est le travail fait dans Solstice après le
basculement : il n'y a pas d'export vers Jira. Fixez une date au-delà de
laquelle vous ne revenez plus, et gardez Jira arrêté mais intact jusque-là.

Si le basculement a échoué dans Solstice lui-même (import échoué ou partiel),
restaurez la base et le répertoire de données de Solstice depuis la sauvegarde
de l'étape 3, ou relancez l'import avec `-replace`. Même quand un import
échoue, Solstice remet ses propres données conservées (voir plus haut), si bien
que l'administrateur qui l'a lancé peut se connecter et réessayer.

## Dépannage

- « the Solstice database already holds data; use -replace to overwrite it » :
  la cible n'est pas vide. Ajoutez `-replace` si c'est bien voulu.
- « another import into this database is running » : attendez la fin de
  l'autre import.
- « this Jira version is too old » : mettez Jira à jour vers la version 7 ou
  plus récente, puis importez.
- Une erreur de connexion qui nomme une option : l'URL de la base porte une
  option que Solstice n'accepte pas (voir la liste plus haut).
- Les dates sont décalées d'une heure ou d'un jour : la valeur de `-tz` n'est
  pas le fuseau horaire du serveur Jira.
