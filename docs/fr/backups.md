# Sauvegardes

Solstice garde son état à trois endroits. Sauvegardez les trois
et restaurez-les ensemble. Pages liées : [installation](install.md),
[reverse proxy](reverse-proxy.md), [mises à jour](upgrades.md).

## Ce qu'il faut sauvegarder

| Quoi | Où | Contenu |
|---|---|---|
| La base de données | MariaDB ou MySQL, désignée par `SOLSTICE_DB_URL` | Projets, tickets, commentaires, utilisateurs, réglages, journal d'audit. |
| Le dossier de données | `SOLSTICE_DATA` : `/var/lib/solstice` sous Linux et dans l'image Docker | `attachments/` (les fichiers joints aux tickets) et `secret.key`. |
| La configuration | `/etc/solstice/solstice.conf` sous Linux ; `solstice.conf` dans le dossier de données avec Docker | Les réglages, dont le mot de passe de la base. |

Remarques :

- Les fichiers joints sont sur le disque, dans `<données>/attachments`, et non
  dans la base.
- Avec Docker, le fichier de configuration est dans le volume `data` : le
  dossier de données le couvre. Gardez aussi `deploy/docker/.env`, qui contient
  les mots de passe de la base, et `compose.yaml` si vous l'avez modifié.
- Sous Linux, le dossier de données contient aussi les dossiers `update/` et
  `backups/` de l'assistant de mise à jour. `backups/` n'est lisible que par
  root et contient les sauvegardes de la base faites avant les mises à jour
  (voir [mises à jour](upgrades.md)). Ce n'est pas un plan de sauvegarde :
  l'assistant n'en fait une que lorsque vous mettez à jour, et garde les 5
  dernières.
- `<données>/setup-code` n'existe que jusqu'à la fin de l'installation.
- Si vous fixez `SOLSTICE_SECRETKEY` dans l'environnement plutôt que dans le
  fichier de configuration, gardez aussi une copie de cette clé.
- Faites l'export de la base et la copie du dossier de données à peu près au
  même moment, pour que les fichiers joints correspondent à la base.
- Gardez `secret.key` à part des exports de la base. Voir
  [ce que protège secret.key](#ce-que-protège-secretkey).

## Exporter la base

L'assistant de mise à jour exporte la base avec ces options, et vous le pouvez
aussi. `mariadb-dump` est l'outil de MariaDB ; avec MySQL, utilisez
`mysqldump`, qui accepte les mêmes options.

```sh
mariadb-dump --single-transaction --quick --hex-blob --no-tablespaces \
  --host=127.0.0.1 --port=3306 --user=solstice --password solstice \
  | gzip > solstice-$(date -u +%Y%m%dT%H%M%SZ).sql.gz
```

L'hôte, le port, l'utilisateur et le nom de la base sont dans
`SOLSTICE_DB_URL`. `--password` demande le mot de passe, qui est dans la même
adresse (encodé en pourcentage : `%40` vaut `@`). Si votre base exige une
liaison chiffrée, ajoutez les options TLS propres à votre outil d'export :
l'option `?tls=` de Solstice ne s'y applique pas.

Avec Docker, lancez l'export dans le conteneur `mariadb`. Il a déjà le nom de
la base, l'utilisateur et le mot de passe dans son environnement. Lancez-le
depuis `deploy/docker` :

```sh
docker compose exec -T mariadb sh -c \
  'MYSQL_PWD="$MARIADB_PASSWORD" mariadb-dump --single-transaction --quick --hex-blob --no-tablespaces --user="$MARIADB_USER" "$MARIADB_DATABASE"' \
  | gzip > solstice.sql.gz
```

## Copier le dossier de données et la configuration

Sous Linux, en root, pour conserver propriétaires et modes :

```sh
tar czf solstice-data.tar.gz -C /var/lib/solstice .
cp -p /etc/solstice/solstice.conf solstice.conf.backup
```

L'archive et la copie contiennent des secrets : conservez-les avec le même
soin que la base.

Avec Docker, copiez le dossier de données hors du conteneur `solstice`. Cela
inclut le fichier de configuration :

```sh
docker compose cp solstice:/var/lib/solstice ./solstice-data
```

## Restaurer

Restaurez avec la même version de Solstice que celle qui a fait l'export, ou
une plus récente. Solstice applique au démarrage les migrations qui manquent.

Sous Linux :

1. Arrêtez Solstice :

   ```sh
   systemctl stop solstice
   ```

2. Repartez d'une base vide. Pour la supprimer et la recréer, avec
   l'instruction qu'utilise l'installateur (cela efface ce que contient la
   base) :

   ```sql
   DROP DATABASE solstice;
   CREATE DATABASE solstice CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
   ```

   Le compte que désigne `SOLSTICE_DB_URL` doit pouvoir l'utiliser, comme
   avant.

3. Chargez l'export :

   ```sh
   gunzip -c solstice-20260101T000000Z.sql.gz \
     | mariadb --host=127.0.0.1 --port=3306 --user=solstice --password solstice
   ```

4. Restaurez le dossier de données et la configuration. Extraite par root,
   l'archive garde les propriétaires avec lesquels elle a été faite
   (`solstice`) :

   ```sh
   tar xzf solstice-data.tar.gz -C /var/lib/solstice
   install -o solstice -g solstice -m 0600 solstice.conf.backup /etc/solstice/solstice.conf
   ```

5. Démarrez Solstice et vérifiez :

   ```sh
   systemctl start solstice
   curl http://127.0.0.1:8080/healthz
   ```

Avec Docker, depuis `deploy/docker` :

1. Arrêtez Solstice en laissant MariaDB tourner :

   ```sh
   docker compose stop solstice
   ```

2. Videz la base et chargez l'export :

   ```sh
   docker compose exec -T mariadb sh -c \
     'MYSQL_PWD="$MARIADB_ROOT_PASSWORD" mariadb --user=root -e "DROP DATABASE solstice; CREATE DATABASE solstice CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"'
   gunzip -c solstice.sql.gz | docker compose exec -T mariadb sh -c \
     'MYSQL_PWD="$MARIADB_PASSWORD" mariadb --user="$MARIADB_USER" "$MARIADB_DATABASE"'
   ```

3. Remettez les fichiers sauvegardés dans le volume `data`. Docker garde le
   volume `solstice_data` ; `docker volume inspect solstice_data` indique où.
   Les fichiers doivent appartenir à l'utilisateur et au groupe 65532, celui
   sous lequel tourne l'image.

4. Démarrez Solstice :

   ```sh
   docker compose start solstice
   ```

Pour restaurer sur un nouveau serveur, installez d'abord Solstice (voir
[installation](install.md)) sans passer par l'assistant d'installation.
Restaurez comme ci-dessus, puis démarrez Solstice. La base restaurée contient
des comptes : Solstice ouvre donc l'application et non l'assistant. Si
l'adresse de la base du nouveau serveur diffère de l'ancienne, gardez le
`SOLSTICE_DB_URL` du nouveau fichier de configuration.

## Ce que protège secret.key

Solstice chiffre les secrets qu'il range dans la base : le mot de passe SMTP
et les secrets des webhooks. Il utilise AES-256-GCM avec une clé de 32 octets,
pour qu'une copie de la base seule ne les révèle pas.

La clé vient de :

1. `SOLSTICE_SECRETKEY`, quand elle est fixée : 32 octets en base64 ou en
   hexadécimal, par exemple issus de `openssl rand -base64 32` ;
2. sinon `<données>/secret.key`. Solstice la crée la première fois qu'il en a
   besoin, avec le mode 0600. Elle contient la clé en base64 : vous pouvez la
   copier dans `SOLSTICE_SECRETKEY`.

Si vous perdez la clé, ces secrets ne sont plus lisibles. Rien d'autre n'est
perdu : ressaisissez le mot de passe SMTP et les secrets des webhooks dans
l'administration. Si `secret.key` manque, Solstice en crée une nouvelle, et
les secrets chiffrés avec l'ancienne restent illisibles.

Comme la clé et les secrets chiffrés doivent rester séparés, ne rangez pas
`secret.key` au même endroit que l'export de la base. Une base restaurée sans
sa clé fonctionne, mais les secrets SMTP et des webhooks sont à ressaisir.
