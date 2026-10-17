# Guide d'administration

Cette page s'adresse aux administrateurs de Solstice. Elle suit le menu de
l'administration (`/admin`), que seuls les comptes dotés du droit
d'administrateur peuvent ouvrir. Elle dit ce que fait chaque partie et ce
qu'il faut surveiller. Elle ne décrit pas chaque bouton.

L'installation, le proxy inverse, les sauvegardes et les mises à jour sont
dans [install.md](install.md), [reverse-proxy.md](reverse-proxy.md),
[backups.md](backups.md) et [upgrades.md](upgrades.md).

Ce qui se règle où :

- Le fichier de configuration et les variables d'environnement ne servent qu'à
  démarrer le serveur : URL de la base, répertoire de données, adresse
  d'écoute. Voir `solstice -h`.
- Tout ce qui suit est stocké dans la base de données et s'applique tout de
  suite, sans redémarrage.
- Chaque changement fait dans l'administration est écrit dans le journal
  d'audit.

## Général

### Vue d'ensemble

Liste ce qu'il reste à régler (aucun projet, e-mail désactivé) et renvoie vers
chaque section.

### Réglages généraux

| Réglage | Remarques |
|---|---|
| Nom de l'instance | 100 caractères au plus. Affiché à côté du logo. |
| Adresse publique | L'adresse à laquelle on ouvre Solstice : `https://solstice.example.org`, sans chemin. Sert aux liens des e-mails. Une adresse `https://` rend les cookies de session `Secure`. |
| Fuseau horaire | Un nom IANA comme `Europe/Paris`. Sert aux dates saisies dans les recherches, aux heures des e-mails et aux résumés quotidiens. Vide : le fuseau du serveur. |
| Langue par défaut | Pour les personnes qui n'en ont pas choisi, et pour les e-mails qu'elles reçoivent. `en`, `fr`, `de`, `it` ou `hi`. |
| Durée de session | De 1 à 365 jours, 30 par défaut. S'applique aux prochaines connexions. |
| Longueur minimale du mot de passe | De 8 à 128, 12 par défaut. S'applique aux nouveaux mots de passe ; les anciens continuent de fonctionner. |

À surveiller :

- L'adresse publique et le fuseau horaire peuvent d'abord venir de
  `SOLSTICE_HTTP_BASEURL` et de `SOLSTICE_TZ`. Une fois enregistrée ici, la
  valeur de la base l'emporte. « Rétablir la valeur d'origine » la supprime et
  revient au fichier, à l'environnement ou à la valeur par défaut.
- Sans adresse publique, les e-mails partent sans liens, et les invitations et
  les liens de réinitialisation de mot de passe ne peuvent pas être envoyés.

### Bandeau d'annonce

Un message en haut de toutes les pages, pour tout le monde. 500 caractères au
plus, avec le ton « information » ou « avertissement ». Chacun peut le
masquer pour soi ; il revient si le texte change. Servez-vous-en pour les
maintenances prévues.

## Utilisateurs et groupes

### Utilisateurs

Chaque personne qui se connecte a un compte. La liste affiche les 200
premiers résultats : affinez la recherche pour trouver les autres.

- **Les identifiants** sont en minuscules : lettres, chiffres et `. _ @ -`,
  en commençant par une lettre ou un chiffre.
- **Créer un compte.** Soit vous choisissez un mot de passe (au moins la
  longueur minimale), soit vous envoyez une invitation par e-mail. Une
  invitation demande une adresse e-mail sur le compte, l'envoi d'e-mails actif
  et une adresse publique. La personne reçoit un lien, valable 7 jours, pour
  choisir son mot de passe. « Renvoyer l'invitation » remplace le lien.
- **Désactiver** un compte ferme ses sessions aussitôt et interdit les
  connexions. Ses tickets, commentaires et historique restent. Vous pouvez le
  réactiver plus tard.
- **Droit d'administrateur.** Donne accès à toute l'administration. Vous ne
  pouvez pas retirer votre propre droit.
- **Changer un mot de passe.** Trois façons : en saisir un, en générer un
  (affiché une seule fois, à transmettre tout de suite), ou envoyer un lien
  par e-mail à la personne. Le lien ne sert qu'une fois et pendant une heure ;
  le mot de passe ne change pas tant qu'il n'est pas utilisé. Saisir ou
  générer un mot de passe ferme les sessions de la personne.
- **Groupes.** La fenêtre du compte permet de choisir les groupes de la
  personne.
- Chacun peut aussi demander un lien de réinitialisation depuis la page de
  connexion, quand les liens par e-mail sont disponibles.
- Les comptes importés d'un annuaire externe de Jira (LDAP et autres) n'ont
  pas de mot de passe. Envoyez-leur un lien, ou définissez-en un.

Si tous les administrateurs sont bloqués, créez ou réparez un compte depuis la
ligne de commande du serveur. Les commandes s'exécutent sur la base configurée
et sont écrites dans le journal d'audit :

```sh
solstice user add -username admin2 -name "Second Admin" -admin
solstice user passwd -username admin2
```

Sans `-password`, un mot de passe est généré et affiché.

### Groupes

Les groupes rassemblent des personnes pour leur donner des droits ensemble. On
les retrouve dans les permissions des projets, dans les rôles de projet, pour
restreindre des commentaires et des saisies de temps, pour partager des
filtres et dans la politique des jetons d'accès. Les noms de groupe sont
stockés en minuscules et sont uniques.

À surveiller :

- Supprimer un groupe retire ses permissions, ses appartenances à des rôles et
  à des niveaux de confidentialité. La suppression est refusée tant que des
  commentaires ou des saisies de temps sont restreints au groupe, car elle les
  cacherait à tout le monde.
- Renommer un groupe conserve les commentaires et les saisies de temps qui lui
  sont restreints.

Dans cette version, la page Groupes du menu n'affiche que son titre. On peut
affecter des groupes aux personnes depuis la fenêtre du compte, et l'API
d'administration crée, renomme et supprime les groupes (`/api/admin/groups`,
décrite dans `backend/internal/api/ADMIN_API.md`).

### Rôles

Les rôles de projet sont communs à tous les projets. Qui tient un rôle se règle
projet par projet, dans les réglages du projet, sous « Rôles dans ce
projet ». On peut ensuite donner des permissions à ce rôle dans le projet,
comme à un groupe.

À surveiller :

- La suppression d'un rôle est refusée tant que des commentaires ou des
  saisies de temps lui sont restreints.
- Comme pour les groupes, la page Rôles du menu n'affiche que son titre dans
  cette version. Les rôles se créent, se renomment et se suppriment par l'API
  d'administration (`/api/admin/roles`).

## Projets

### Page Projets

Liste tous les projets avec leur clé, leur responsable, leur catégorie, leur
nombre de tickets et leur état, et renvoie vers les réglages de chacun.

- **Créer un projet.** La clé compte de 2 à 10 majuscules ou chiffres et
  commence par une lettre. Elle ne peut plus être changée ensuite, et elle doit
  différer de toutes les clés actuelles et anciennes. Vous choisissez aussi le
  workflow de départ (Logiciel : To Do, In Progress, In Review, Done ; ou
  Simple : trois statuts), le responsable (vous par défaut), qui peut travailler
  dans le projet (le responsable seul, tous les titulaires d'un compte, un
  groupe ou un rôle), les types de ticket et l'assigné par défaut. Le
  responsable reçoit toutes les permissions. Le premier projet d'une instance
  neuve remplit aussi les catalogues vides avec des valeurs par défaut, dans la
  langue de l'instance.
- **Archiver** met un projet en lecture seule et le retire des listes et des
  sélecteurs de projets. Rien n'est perdu, et vous pouvez le désarchiver à tout
  moment. Préférez-le à la suppression.
- **Supprimer** est définitif : tickets, commentaires, pièces jointes et leurs
  fichiers, historique, composants, versions, workflows et permissions
  disparaissent. Un projet qui contient des tickets demande de saisir sa clé.
  Les tableaux et les filtres enregistrés restent, et peuvent n'afficher rien.
- **Les catégories** regroupent les projets dans la liste. Supprimer une
  catégorie laisse ses projets sans catégorie.

### Réglages d'un projet

Chaque projet a ses propres règles. Rien n'est partagé par des schémas : un
changement ne touche aucun autre projet. Seuls les administrateurs du projet
(et les administrateurs globaux) peuvent ouvrir les réglages. La page compte
deux onglets.

**Permissions et rôles.** Un tableau de ce que chaque titulaire peut faire. Les
permissions sont : voir, créer, modifier, commenter, changer le statut,
assigner, être assigné, joindre des fichiers, lier, saisir du temps,
supprimer, modérer, gérer les observateurs, confidentialité et administrer.
Une permission s'accorde à tout le monde, à un utilisateur, à un groupe, à un
rôle, au responsable du projet, au rapporteur ou à l'assigné.

À surveiller :

- « Administrer » permet de régler le projet. Les administrateurs globaux
  règlent tous les projets, mais ne voient pas le contenu des tickets si une
  permission ne le leur accorde pas.
- « Modérer » permet de modifier ou retirer les commentaires, les saisies de
  temps et les fichiers des autres. Accordez-la avec parcimonie.
- Une permission sans titulaire n'est accordée à personne. Après un import
  Jira, lisez le rapport d'import : les permissions que Solstice n'a pas su
  traduire sont retirées, jamais élargies.

**Workflows.** Voir ci-dessous.

### Workflows

Un workflow fixe les statuts d'un ticket et les passages permis entre eux. Les
workflows appartiennent à un projet. Chaque type de ticket suit l'un des
workflows du projet. Le workflow par défaut prend tous les types qui n'en
suivent pas d'autre.

- Un workflow a un statut initial (où démarrent les nouveaux tickets), une
  liste de statuts et un tableau de transitions. Cochez une case pour permettre
  le passage du statut de la ligne à celui de la colonne. Une ligne « Tout
  statut » permet un passage depuis n'importe où. Le nom de la transition est
  celui du bouton affiché sur le ticket.
- Vous pouvez créer un workflow à partir de la copie d'un autre.
- Les statuts sont communs à tous les projets, et leurs noms aussi. Ajouter un
  statut qui n'existe pas le crée, avec une catégorie.
- Quand vous retirez un statut dans lequel se trouvent des tickets, ou
  supprimez un workflow qui laisse des tickets dans des statuts absents du
  workflow par défaut, une fenêtre demande où ces tickets vont. Chaque
  déplacement est inscrit dans l'historique du ticket.
- Supprimer un workflow renvoie ses types de ticket au workflow par défaut.
  Les tickets gardent leur statut actuel.
- Quand un ticket atteint un statut de la catégorie « Terminé » sans
  résolution, il reçoit la résolution par défaut si elle est définie.
- Les conditions, validateurs et post-fonctions de Jira n'existent pas dans
  Solstice. Ce sont les permissions qui décident de qui peut faire une
  transition.
- Sans la permission « Administrer », les workflows sont en lecture seule.

## Tickets

Ces catalogues sont communs à tous les projets. Chaque liste est dans l'ordre
d'affichage : glissez les lignes ou utilisez les flèches. Videz la recherche
pour changer l'ordre.

### Types de ticket

Un nom, une description et l'indication « sous-tâche ». À surveiller :

- La case sous-tâche ne peut changer que tant qu'aucun ticket n'utilise le
  type.
- Supprimer un type utilisé par des tickets, ou proposé par un projet, demande
  de déplacer ses tickets vers un autre type du même genre (sous-tâche ou non).
  Le dernier type ne peut pas être supprimé.

### Priorités

Un nom, une description et une couleur (`#rrggbb`). Une priorité est celle par
défaut, donnée aux tickets créés sans priorité. Sans priorité par défaut, la
priorité du milieu est utilisée. Supprimer une priorité utilisée demande où
déplacer ses tickets.

### Résolutions

Comment un ticket a été clos. La résolution par défaut est donnée aux tickets
qui atteignent un statut terminé sans en avoir. Sans résolution par défaut,
ils restent non résolus. Supprimer une résolution utilisée demande où déplacer
ses tickets.

### Statuts

Chaque statut a une catégorie : à faire, en cours ou terminé. Les tableaux et
les rapports se servent de la catégorie, quel que soit le nom du statut. Un
statut utilisé ne peut pas être supprimé. « Où est-il utilisé » liste les
workflows, les colonnes de tableau et les tickets qui le portent ; retirez-le
d'abord de ces endroits. Un statut supprimé garde son nom dans l'historique des
tickets.

### Types de lien

Un nom et deux verbes, par exemple « bloque » et « est bloqué par ».
Supprimer un type de lien utilisé demande vers quel type déplacer ses liens.

### Champs personnalisés

Des champs ajoutés aux tickets, en plus de ceux d'origine. Types : texte d'une
ligne, texte de plusieurs lignes, lien, choix (une ou plusieurs valeurs),
boutons radio, cases à cocher, nombre, date, date et heure, étiquettes, un
utilisateur, plusieurs utilisateurs.

- Un champ s'applique à tous les projets et types de ticket, ou seulement à
  ceux que vous choisissez. Il apparaît sur un ticket quand son projet et son
  type correspondent tous deux.
- Le type d'un champ ne peut pas changer après la création.
- Les options peuvent être ajoutées, renommées, réordonnées et désactivées.
  Une option désactivée garde ses valeurs existantes mais ne peut plus être
  choisie. Préférez désactiver à supprimer.
- Supprimer une option utilisée par des tickets demande de déplacer leur
  valeur vers une autre option ou de l'effacer.
- Supprimer un champ qui contient des valeurs demande de saisir son nom. Les
  valeurs disparaissent avec lui et ne peuvent pas être récupérées. Les
  tableaux qui estiment avec ce champ n'ont plus d'estimation, et les filtres
  enregistrés qui le nomment ne trouvent plus rien.
- Les champs importés de Jira gardent leur type Jira. Un type que Solstice ne
  sait pas modifier est indiqué « type Jira, affichage seul ». Un champ en
  cascade importé de Jira n'affiche que son premier niveau d'options.

## E-mail et intégrations

### E-mail

Solstice envoie les notifications, les résumés quotidiens, les abonnements aux
filtres, les rappels, les invitations et les liens de réinitialisation de mot
de passe par un seul serveur SMTP.

- Réglez le serveur, le port et la sécurité de la connexion : STARTTLS (port
  587, le choix usuel), TLS dès le départ (port 465) ou aucune (port 25, pour
  un relais sur un réseau interne). Ajoutez un identifiant et un mot de passe
  si le serveur en demande, et une adresse d'expédition.
- Le mot de passe est stocké chiffré avec la clé secrète. Sauvegardez cette clé
  avec la base (voir [backups.md](backups.md)).
- Si vous changez le serveur, le port, la sécurité ou l'identifiant, saisissez
  de nouveau le mot de passe ou supprimez-le.
- « Envoyer un e-mail de test » utilise les réglages enregistrés et affiche la
  réponse exacte du serveur en cas d'échec. Faites-le après chaque changement.
- La file d'envoi montre les messages en attente, en échec et récemment
  envoyés. « Relancer les envois en échec » sert après avoir corrigé les
  réglages.
- Décocher « Envoyer des e-mails » conserve les réglages mais arrête tout
  envoi.

### Webhooks

Un webhook envoie un POST au format des webhooks de Jira à un autre outil
quand quelque chose change.

- Choisissez l'adresse, les événements (tickets, commentaires, temps saisi,
  sprints, tableaux) et, si vous voulez, une requête JQL : seuls les tickets
  qui y répondent déclenchent un envoi.
- Avec un secret, chaque envoi porte `X-Hub-Signature: sha256=…`, un
  HMAC-SHA256 du corps. Le destinataire doit le vérifier. Le secret est stocké
  chiffré.
- « Envoyer un corps vide » convient à un service qui ne vérifie que la
  signature.
- « Envoyer un appel de test » envoie un POST à l'adresse aussitôt et affiche
  la réponse.
- Un envoi en échec est retenté jusqu'à 6 fois, après 30 secondes, 2 minutes,
  10 minutes, 30 minutes et 2 heures. Chaque webhook garde ses 100 derniers
  envois, avec la réponse de chacun.
- Les webhooks en échec ces dernières 24 heures apparaissent sur la page
  système.

À surveiller :

- Un webhook voit tous les tickets, y compris les confidentiels. Limitez-le
  avec sa requête JQL.
- Les cibles peuvent se trouver sur votre réseau privé ou sur la machine
  locale. Les adresses de lien local et les adresses de métadonnées des
  fournisseurs de cloud sont toujours refusées, les redirections ne sont pas
  suivies et aucun proxy n'est utilisé.

### Jetons d'accès

Les jetons d'accès personnels permettent à des scripts, des tâches d'intégration
continue et des extensions d'IDE de se connecter comme une personne, sans son
mot de passe. Chacun crée les siens dans ses réglages (voir
[rest-api.md](rest-api.md)). Cette page liste les jetons de tous les comptes,
avec leur dernier usage et leur expiration, et en révoque n'importe lequel
aussitôt.

La politique « Qui peut créer des jetons » :

- **Les administrateurs seulement** (par défaut).
- **Les membres d'un groupe.**
- **Tout le monde.**

Les administrateurs le peuvent toujours. Durcir la politique ne révoque pas les
jetons existants : révoquez-les depuis la liste. Un jeton ne peut pas créer
d'autres jetons. Réinitialiser un mot de passe par un lien de réinitialisation
supprime aussi les jetons du compte.

## Système

### Journal d'audit

Un registre en lecture seule de qui s'est connecté et de qui a modifié les
comptes, les droits d'accès, les projets, les workflows, les catalogues, les
réglages, les webhooks, les jetons, les imports et les mises à jour, avec
l'adresse et l'heure. Filtrez par type d'action, utilisateur, dates ou mot de
la cible ou des détails. Ouvrez une entrée pour voir le détail, comme l'ancienne
et la nouvelle valeur d'un réglage.

Rétention : les entrées plus anciennes que la durée de rétention sont
supprimées automatiquement, une fois par heure. La valeur par défaut est de 365
jours. Choisissez 30 jours ou plus, ou 0 pour tout garder. Raccourcir la durée
supprime aussitôt les entrées plus anciennes, après une confirmation, et c'est
irréversible. Faites d'abord une sauvegarde de la base si vous avez besoin des
entrées anciennes.

### Import Jira

Remplace tout ce que contient Solstice par les données d'une base Jira. C'est
la page la plus destructrice de l'administration. Lisez
[migration.md](migration.md) avant de l'utiliser. Elle demande de confirmer
l'existence d'une sauvegarde et de saisir le nom de la base. Solstice est en
maintenance pendant l'import.

### Informations système

La version, la version de Go, l'heure de démarrage, le serveur, le nom et la
taille de la base, le répertoire de données et la place qu'il occupe, le
fichier de configuration et l'adresse d'écoute, le nombre de comptes, de
projets, de tickets, de commentaires et de pièces jointes, la file d'e-mails,
les webhooks en échec, les migrations de base appliquées et un import en cours.
Aucun secret n'y figure. Regardez-y en premier quand quelque chose semble
anormal : un nombre croissant d'e-mails en échec ou un webhook en échec s'y
voit.

### Mises à jour

Affiche la version installée et la dernière version publiée sur GitHub.

- Solstice cherche les nouvelles versions au plus toutes les 12 heures, et
  n'envoie que la demande du numéro de la dernière version. Vous pouvez
  désactiver les vérifications automatiques, ou utiliser « Vérifier
  maintenant ». C'est la seule requête que Solstice fait de lui-même.
- Sur une installation Linux faite avec l'installateur, « Mettre à jour »
  télécharge la version, la vérifie, sauvegarde la base, redémarre, et remet la
  version précédente si la nouvelle ne démarre pas. Vous confirmez avec votre
  mot de passe. Solstice est indisponible quelques secondes ; les personnes
  connectées le restent.
- Dans un conteneur ou après une installation manuelle, la page affiche les
  commandes à exécuter.

Voir [upgrades.md](upgrades.md) avant de mettre à jour une instance de
production.
