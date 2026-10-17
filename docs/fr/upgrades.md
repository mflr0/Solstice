# Mises à jour

Solstice migre sa base tout seul au démarrage : une mise à jour consiste donc
à installer un nouveau binaire (ou une nouvelle image) et à redémarrer. Pages
liées : [installation](install.md), [reverse proxy](reverse-proxy.md),
[sauvegardes](backups.md).

Sauvegardez avant de mettre à jour, comme décrit dans
[sauvegardes](backups.md). La mise à jour intégrée exporte la base pour vous ;
les autres méthodes ne le font pas.

Pour connaître la version en cours, utilisez `solstice version`, la page des
mises à jour de l'administration, ou `GET /healthz`, qui répond
`{"status":"ok","version":...}`.

## Mise à jour intégrée (Linux, systemd)

Sur une installation faite avec le `install.sh` de l'archive publiée, les
administrateurs globaux peuvent mettre à jour depuis l'administration, sur la
page Mises à jour.

Solstice interroge GitHub pour savoir si une nouvelle version existe, au plus
toutes les 12 heures. C'est la seule requête qu'il fait de lui-même, et un
administrateur peut désactiver ces vérifications. La page montre la version
installée, la dernière version publiée et ses notes. **Mettre à jour vers
v<version>** demande à l'assistant de mise à jour de l'installer. La page ne
propose ce bouton que si l'assistant est installé. Dans un conteneur Docker ou
sur une installation sans assistant, elle montre à la place les commandes à
lancer.

### Ce que fait l'assistant

Solstice tourne sous l'utilisateur non privilégié `solstice`. Tout ce qu'il
fait est d'écrire un fichier de demande qui nomme une version. L'assistant
tourne en root et fait le travail, dans cet ordre :

1. Vérifie que la version est un numéro de version valide et plus récent que
   celui en cours. Il n'installe jamais une version plus ancienne.
2. Télécharge `SHA256SUMS`, `SHA256SUMS.sig` et l'archive de cette machine
   (`amd64` ou `arm64`) depuis la publication GitHub, en HTTPS.
3. Vérifie la signature de `SHA256SUMS`. C'est une signature Ed25519 de la clé
   de publication écrite dans `/usr/local/lib/solstice/update.sh`. Aucune somme
   de contrôle n'est lue avant que la signature soit valide. Une version dont
   la signature manque ou est fausse est refusée.
4. Vérifie la somme de contrôle de l'archive dans `SHA256SUMS`.
5. Extrait le binaire et vérifie qu'il annonce la version demandée.
6. Sauvegarde la base (voir plus bas).
7. Garde le binaire en cours sous `/usr/local/lib/solstice/solstice.previous`.
8. Remplace `/usr/local/bin/solstice` et redémarre le service `solstice`.
9. Attend jusqu'à 60 secondes que le service tourne et réponde à sa
   vérification d'état.
10. Sinon, remet le binaire précédent et le redémarre. L'état est alors
    `rolled_back`.

Un échec à l'une des étapes jusqu'à la 8 laisse la version installée
inchangée.

L'assistant a besoin de `curl` ou `wget`, d'`openssl` 1.1.1 ou plus récent
(pour Ed25519), et de `setpriv` ou `runuser`. Il refuse toute mise à jour tant
que la clé de publication de `update.sh` est encore le texte de substitution.
La clé est remplacée quand vous lancez `install.sh` d'une archive plus
récente, pas par les mises à jour elles-mêmes.

### La sauvegarde

Avant d'installer quoi que ce soit, l'assistant exporte la base dans
`/var/lib/solstice/backups/<heure UTC>.sql.gz` et garde les 5 dernières. Le
dossier appartient à root, mode 0700. L'export tourne sous l'utilisateur
`solstice` avec `mariadb-dump` ou `mysqldump`, avec `--single-transaction
--quick --hex-blob --no-tablespaces`, en TCP.

- Si aucun des deux outils n'est installé, l'assistant le dit dans son journal
  et met à jour sans sauvegarde.
- Si aucune base n'est configurée, il n'y a rien à sauvegarder.
- Si l'export échoue, ou si `SOLSTICE_DB_URL` est illisible, l'assistant
  refuse de mettre à jour.

La sauvegarde couvre la base seulement, pas le dossier de données ni la
configuration.

### Retour en arrière

Si la nouvelle version n'est pas en bonne santé au bout de 60 secondes,
l'assistant remet le binaire précédent. Il n'annule pas les changements faits
à la base, car MariaDB n'annule pas les changements de schéma. Le journal de
l'assistant indique alors la sauvegarde faite avant la mise à jour. Pour
revenir complètement en arrière, restaurez-la comme décrit dans
[sauvegardes](backups.md).

### État et journaux

L'assistant écrit sa progression dans `/var/lib/solstice/update/status`, que
lit la page Mises à jour. L'état est `running`, `ok`, `failed` ou
`rolled_back`. Les mêmes lignes vont dans le journal systemd :

```sh
journalctl -u solstice-update
```

### Les unités de l'assistant de mise à jour

`install.sh` installe deux unités systemd à côté de `solstice.service`, et
`update.sh` dans `/usr/local/lib/solstice` :

- `solstice-update.path` surveille `/var/lib/solstice/update/request`. Quand
  ce fichier apparaît, elle démarre `solstice-update.service`. `install.sh`
  l'active.
- `solstice-update.service` est un service à exécution unique qui lance
  `/usr/local/lib/solstice/update.sh` en root, avec 30 minutes au plus pour
  finir. Il ne peut écrire que dans `/usr/local/bin`,
  `/usr/local/lib/solstice` et `/var/lib/solstice`.

Le dossier `/var/lib/solstice/update` appartient à root, groupe `solstice`,
mode 1770 : Solstice peut y déposer une demande mais ne peut pas toucher aux
fichiers de root. Le fichier `/var/lib/solstice/update/helper`, écrit par
`install.sh`, indique à Solstice que l'assistant est installé.

Vérifiez les unités :

```sh
systemctl status solstice-update.path
```

## Mise à jour manuelle (Linux)

Utilisez-la quand l'assistant n'est pas installé, quand il refuse une mise à
jour, ou quand vous préférez maîtriser les étapes.

```sh
curl -fLO https://github.com/mflr0/Solstice/releases/download/v<version>/solstice-<version>-linux-amd64.tar.gz
tar -xzf solstice-<version>-linux-amd64.tar.gz
cd solstice-<version>-linux-amd64
sudo ./install.sh
```

Écrivez `arm64` dans le nom sur une machine ARM. Relancer `install.sh` garde la
configuration et les données, remplace le binaire, l'unité et l'assistant de
mise à jour, redémarre le service et migre la base. Il ne sauvegarde rien :
faites-le d'abord.

Sans systemd, remplacez le binaire `solstice` et relancez `solstice serve` de
la façon dont vous le lancez. Il migre la base au démarrage ; `solstice
migrate` ne fait que cela.

## Mise à jour avec Docker

L'image Docker est construite à partir des sources. Pour passer à une version
publiée, placez-vous sur son étiquette et relancez l'installateur, depuis le
dépôt :

```sh
git fetch --tags
git checkout v<version>
SOLSTICE_VERSION=<version> ./deploy/docker/install.sh
```

L'installateur garde `.env` et les deux volumes, reconstruit l'image
(`docker compose up -d --build`) et redémarre les conteneurs. Solstice migre la
base quand il démarre. `SOLSTICE_VERSION` est l'étiquette de l'image et la
version qu'annonce la construction.

L'installateur n'affiche un code d'installation que pour une instance sans
compte. Pour une instance déjà installée, il affiche l'adresse.

## L'option `tls` et la sauvegarde

`SOLSTICE_DB_URL` accepte une option, `?tls=` (voir
[installation](install.md)), qui chiffre la liaison propre de Solstice avec la
base. L'assistant de mise à jour lit l'adresse de la base dans le fichier de
configuration mais ignore tout ce qui suit le `?`. Il ne passe aucune option
TLS à l'outil d'export. Deux conséquences :

- Que la liaison de la sauvegarde soit chiffrée ou non dépend des valeurs par
  défaut de l'outil d'export, et non de `?tls=`.
- Si le serveur de base refuse les connexions que fait l'outil d'export,
  l'export échoue et l'assistant refuse de mettre à jour, avec le message
  « The database backup failed: refusing to update without one. »

Dans ce cas, faites vous-même un export avec les options TLS de votre outil
(voir [sauvegardes](backups.md)), puis mettez à jour avec la méthode manuelle
ci-dessus.

Il en va de même pour une adresse de base que l'assistant ne sait pas lire :
il refuse de mettre à jour sans sauvegarde. L'assistant ne lit que les
adresses `mysql://` et `mariadb://`.
