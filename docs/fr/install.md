# Installation

Cette page décrit l'installation de Solstice sur un serveur Linux avec
systemd, ou avec Docker, puis l'assistant d'installation et la configuration.
Pages liées : [reverse proxy](reverse-proxy.md), [sauvegardes](backups.md),
[mises à jour](upgrades.md).

Solstice est un seul binaire. Il range ses données dans MariaDB 10.6+ ou
MySQL 8.0+ et dans un dossier de données sur le disque (pièces jointes, clé
des secrets enregistrés, code d'installation). Il écoute en HTTP simple :
placez un reverse proxy devant lui pour servir en HTTPS.

## Linux (systemd)

Téléchargez une archive publiée, extrayez-la et lancez l'installateur en
root :

```sh
tar xzf solstice-<version>-linux-amd64.tar.gz
cd solstice-<version>-linux-amd64
sudo ./install.sh --with-mariadb
```

L'installateur exige systemd et accepte ces options :

| Option | Effet |
|---|---|
| `--with-mariadb` | Installe MariaDB depuis les paquets de la distribution si aucun client `mariadb` ou `mysql` n'est trouvé (`apt-get`, `dnf` ou `zypper`), crée la base `solstice` et son compte avec un mot de passe aléatoire, et écrit `SOLSTICE_DB_URL` dans le fichier de configuration. L'assistant d'installation saute alors son étape de la base. |
| `--port N` | Port d'écoute de Solstice. Défaut : `8080`. Écrit sous la forme `SOLSTICE_HTTP_ADDR=:N` quand l'installateur crée le fichier de configuration. |
| `--binary CHEMIN` | Le binaire `solstice` à installer. Défaut : celui qui se trouve à côté du script. |

Sans `--with-mariadb`, créez la base vous-même et indiquez son adresse à
l'assistant d'installation. L'installateur utilise cette instruction, que vous
pouvez adapter :

```sql
CREATE DATABASE solstice CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### Ce que fait l'installateur

- Il crée l'utilisateur système `solstice`.
- Il installe le binaire dans `/usr/local/bin/solstice`.
- Il crée `/etc/solstice/solstice.conf` (mode 0600, propriétaire `solstice`)
  s'il n'existe pas.
- Il crée `/var/lib/solstice`, le dossier de données.
- Il installe l'unité systemd `/etc/systemd/system/solstice.service`, puis
  l'active et la démarre.
- Il installe l'assistant de mise à jour quand `update.sh`,
  `solstice-update.path` et `solstice-update.service` se trouvent à côté du
  script, comme dans les archives publiées : `/usr/local/lib/solstice/update.sh`
  et les deux unités dans `/etc/systemd/system`. Voir
  [mises à jour](upgrades.md).
- Il attend que Solstice réponde, puis affiche l'adresse de l'assistant
  d'installation et le code d'installation.

Relancer l'installateur avec une archive plus récente met à jour sur place :
la configuration et les données sont conservées, le service redémarre et la
base est migrée au démarrage.

### Le service

`solstice.service` lance `/usr/local/bin/solstice serve` sous l'utilisateur
`solstice`. Il définit `SOLSTICE_CONFIG=/etc/solstice/solstice.conf` et
`SOLSTICE_DATA=/var/lib/solstice` dans son environnement. Les variables
d'environnement l'emportent sur le fichier : une ligne `SOLSTICE_DATA` dans le
fichier n'a donc aucun effet avec cette unité.

L'unité relance Solstice après un échec (`Restart=on-failure`, au bout de
5 secondes) et laisse 30 secondes pour l'arrêt (`TimeoutStopSec=30`). Elle
démarre après `network-online.target`, `mariadb.service` et `mysql.service`.
Elle est durcie : aucun nouveau privilège, aucune capacité, le système en
lecture seule sauf `/var/lib/solstice` et `/etc/solstice`, et les dossiers
personnels en lecture seule.

```sh
systemctl status solstice
journalctl -u solstice -f
```

### Désinstallation

```sh
systemctl disable --now solstice solstice-update.path
```

Supprimez ensuite les fichiers listés plus haut, l'utilisateur système
`solstice` et, si vous l'avez créée, la base.

## Docker

Le fichier Compose `deploy/docker/compose.yaml` construit l'image à partir
des sources et la démarre avec un conteneur MariaDB 11.4.

```sh
git clone https://github.com/mflr0/Solstice.git
cd Solstice/deploy/docker
./install.sh
```

`install.sh` exige Docker et Docker Compose v2. Il fait ceci :

1. Il écrit `.env` à côté du fichier Compose s'il n'existe pas, avec des
   `DB_PASSWORD` et `DB_ROOT_PASSWORD` aléatoires (hexadécimaux, mode 0600) et
   `SOLSTICE_PORT` (défaut `8080`). Gardez ce fichier.
2. Il lance `docker compose up -d --build`.
3. Il attend Solstice, puis affiche l'adresse de l'assistant d'installation et
   le code d'installation.

Pour le faire à la main, créez `.env` avec `DB_PASSWORD` et `DB_ROOT_PASSWORD`
(lettres et chiffres uniquement), lancez `docker compose up -d`, et lisez le
code d'installation avec `docker compose logs solstice`.

Ce que met en place le fichier Compose :

- Deux services, `mariadb` et `solstice`, et deux volumes : `db` (la base) et
  `data` (monté sur `/var/lib/solstice`, le dossier de données).
- `SOLSTICE_DB_URL` désigne le service `mariadb` : l'assistant d'installation
  saute donc son étape de la base.
- L'image définit `SOLSTICE_DATA=/var/lib/solstice`,
  `SOLSTICE_CONFIG=/var/lib/solstice/solstice.conf`,
  `SOLSTICE_HTTP_ADDR=:8080`, `SOLSTICE_INSTALL=docker` et
  `SOLSTICE_LOG_FORMAT=json`. Le fichier de configuration se trouve donc dans
  le volume `data`.
- Le port est publié sous la forme `${SOLSTICE_PORT:-8080}:8080`. Utilisez
  plutôt `"127.0.0.1:8080:8080"` si seul un reverse proxy de la même machine
  doit atteindre Solstice.
- L'image a une vérification d'état qui lance `solstice health` toutes les
  30 secondes.
- Pour importer depuis Jira dans l'assistant, décommentez la ligne de volume
  `jira-home` de `compose.yaml`, puis donnez `/import/jira-home` comme chemin
  dans l'assistant.

Pour fixer un autre réglage `SOLSTICE_*`, ajoutez-le sous `environment:` du
service `solstice` dans `compose.yaml`.

```sh
docker compose -f compose.yaml logs -f solstice
docker compose -f compose.yaml down
```

## L'assistant d'installation

Tant qu'aucune base n'est configurée, ou que la base n'a encore aucun compte,
Solstice ne sert que l'assistant d'installation. Les pages redirigent vers
`/setup` et `/api` répond 503 jusqu'à la fin de l'installation. `/healthz`
continue de répondre.

L'assistant est verrouillé par un code à usage unique de huit caractères,
écrit par exemple `4F7K-9QX2`. Solstice :

- l'affiche dans un cadre dans son journal ;
- l'écrit dans `<données>/setup-code` (mode 0600), soit
  `/var/lib/solstice/setup-code` sous Linux ;
- en tire un nouveau à chaque démarrage, tant que l'installation n'est pas
  terminée ;
- supprime le fichier quand le premier administrateur existe.

Quiconque peut lire le journal ou le dossier de données devient
administrateur : protégez-les. Les codes erronés sont limités par adresse de
client.

Les étapes :

1. Ouvrez `/setup` et saisissez le code.
2. Base de données : demandée seulement si `SOLSTICE_DB_URL` n'est pas fixée.
   L'assistant teste la connexion avant de l'enregistrer.
3. Adresse publique et fuseau horaire. L'adresse publique commence par
   `http://` ou `https://`, suivi d'un hôte et d'un port facultatif, sans
   chemin, par exemple `https://solstice.example.org`. Le fuseau horaire est
   un nom IANA, par exemple `Europe/Paris`.
4. Créez l'administrateur, ou importez une instance Jira.

L'assistant écrit `SOLSTICE_DB_URL`, `SOLSTICE_DATA`, `SOLSTICE_HTTP_BASEURL`
et `SOLSTICE_TZ` dans le fichier de configuration. Si le fichier ne peut pas
être écrit, il affiche les lignes à y coller. Quand le premier administrateur
est créé, Solstice passe à l'application sans redémarrage.

## Configuration

Les réglages sont des lignes `CLÉ=valeur` dans un fichier. Solstice cherche
le fichier dans cet ordre et prend le premier qui existe :

1. le chemin de la variable d'environnement `SOLSTICE_CONFIG` ;
2. `./solstice.conf` ;
3. `/etc/solstice/solstice.conf`.

Règles du fichier :

- Les lignes vides et celles qui commencent par `#` sont ignorées.
- `export ` devant une clé est permis.
- Une valeur entourée de guillemets simples ou doubles identiques les perd.
- Si une clé apparaît deux fois, la dernière ligne l'emporte.
- Une valeur vide compte comme non fixée.
- Le même fichier peut être lu par systemd (`EnvironmentFile`) et par Docker
  (`--env-file`). L'assistant n'y écrit donc que des valeurs sans guillemets,
  sans barres obliques inverses, sans `$`, sans accent grave et sans
  caractères de contrôle, et sans espaces au début ni à la fin.

Les variables d'environnement du même nom l'emportent sur le fichier.
Solstice lit le fichier au démarrage : redémarrez-le après une modification.
Une clé `SOLSTICE_` que Solstice ne connaît pas, dans le fichier ou
l'environnement, est signalée dans le journal au démarrage. `solstice -h`
liste les réglages.

Les durées s'écrivent `30s`, `5m`, `1h30m`.

| Clé | Défaut | Signification |
|---|---|---|
| `SOLSTICE_DB_URL` | aucune | La base, `mysql://utilisateur:mot_de_passe@hôte:3306/base`. Voir plus bas. |
| `SOLSTICE_DB_MAXOPENCONNS` | `20` | Connexions ouvertes au plus. Au moins 1. |
| `SOLSTICE_DB_MAXIDLECONNS` | `10` | Connexions inactives gardées. Entre 0 et `SOLSTICE_DB_MAXOPENCONNS`. |
| `SOLSTICE_DB_CONNMAXLIFETIME` | `5m` | Durée de vie d'une connexion. Doit être positive. |
| `SOLSTICE_DATA` | `./data` | Dossier de données : pièces jointes, `secret.key`, `setup-code`. |
| `SOLSTICE_TZ` | celui du système | Fuseau des dates tapées dans les recherches, par exemple `Europe/Paris`. |
| `SOLSTICE_SECRETKEY` | `<données>/secret.key` | Clé des secrets enregistrés. Voir [sauvegardes](backups.md). |
| `SOLSTICE_INSTALL` | aucune | `docker` dans l'image : la page des mises à jour explique comment mettre à jour un conteneur. |
| `SOLSTICE_HTTP_ADDR` | `:8080` | Adresse d'écoute. |
| `SOLSTICE_HTTP_BASEURL` | aucune | Adresse publique, par exemple `https://solstice.example.org`. Voir [reverse proxy](reverse-proxy.md). |
| `SOLSTICE_HTTP_TRUSTEDPROXIES` | aucun | Reverse proxies dont l'en-tête `X-Forwarded-For` est cru. Voir [reverse proxy](reverse-proxy.md). |
| `SOLSTICE_HTTP_SHUTDOWNTIMEOUT` | `10s` | Temps laissé aux requêtes et e-mails en cours quand Solstice s'arrête. Doit être positif. |
| `SOLSTICE_HTTP_MAXUPLOAD` | `100MiB` | Taille d'un envoi de fichiers. `KB`, `MB`, `GB` comptent par 1000, `KiB`, `MiB`, `GiB` par 1024. Montez aussi la limite du proxy. |
| `SOLSTICE_LOG_LEVEL` | `info` | `debug`, `info`, `warn` ou `error`. |
| `SOLSTICE_LOG_FORMAT` | `text` | `text` ou `json` (`json` dans l'image Docker). |

`SOLSTICE_CONFIG` n'est lue que dans l'environnement. Il n'y a pas de base par
défaut : sans `SOLSTICE_DB_URL`, l'assistant en demande une.

### L'adresse de la base

`SOLSTICE_DB_URL` s'écrit `mysql://utilisateur:mot_de_passe@hôte:3306/base`.
Le schéma `mariadb://` est accepté aussi. Encodez en pourcentage les
caractères spéciaux du mot de passe : `@` devient `%40`.

Pour chiffrer la liaison avec une base située sur une autre machine, ajoutez
une option `tls` :

| Option | Effet |
|---|---|
| `?tls=true` | Chiffré, certificat vérifié. |
| `?tls=skip-verify` | Chiffré, certificat non vérifié. |
| `?tls=preferred` | Chiffré quand le serveur le propose, certificat non vérifié. |
| `?tls=false` | Non chiffré. C'est le défaut. |

Aucune autre option après `?` n'est acceptée et Solstice refuse de démarrer
avec une autre. Les messages d'erreur n'affichent jamais le mot de passe.
L'effet de `?tls=` sur la sauvegarde de l'assistant de mise à jour est décrit
dans [mises à jour](upgrades.md#loption-tls-et-la-sauvegarde).

### Vérification d'état

`GET /healthz` répond 200 quand la base répond, 503 sinon. Il répond aussi
pendant l'installation. `solstice health` fait cette requête sur
`SOLSTICE_HTTP_ADDR` et se termine avec le statut 0 si la réponse est 200 ;
`-url` donne une autre adresse.

### Autres commandes

```sh
solstice migrate                          # met à jour le schéma de la base
solstice user add -username me -admin     # crée un compte (mot de passe affiché)
solstice user passwd -username me         # change un mot de passe
solstice version
```

`solstice serve` migre la base au démarrage : `migrate` sert donc rarement.
Lancez ces commandes avec la même configuration que le service, par exemple
avec `SOLSTICE_CONFIG=/etc/solstice/solstice.conf` dans l'environnement.
