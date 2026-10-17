# Reverse proxy

Solstice écoute en HTTP simple et n'a aucun réglage pour les certificats.
Placez un reverse proxy devant lui pour servir en HTTPS. Pages liées :
[installation](install.md), [sauvegardes](backups.md),
[mises à jour](upgrades.md).

Le dépôt contient deux exemples à copier et à adapter :
`deploy/proxy/Caddyfile` et `deploy/proxy/nginx.conf`. Caddy est le plus court
car il obtient et renouvelle lui-même le certificat.

## Ce que le proxy doit faire

- Rediriger le HTTP simple vers HTTPS.
- Transmettre l'en-tête `Host` d'origine.
- Envoyer `X-Forwarded-Proto` (`https`) et `X-Forwarded-For`. Solstice marque
  son cookie de session `Secure` quand il voit `X-Forwarded-Proto: https`.
- Envoyer `Strict-Transport-Security` (HSTS) sur le site HTTPS, pour que les
  navigateurs continuent d'utiliser HTTPS. Ne l'envoyez jamais sur la
  redirection HTTP simple.
- Garder `GET /api/events` ouvert, sans mise en mémoire tampon et sans le
  fermer au bout de 60 secondes. C'est le flux des mises à jour en direct.
- Accepter des corps de requête jusqu'à la taille d'un envoi, 100 MiB par
  défaut (voir `SOLSTICE_HTTP_MAXUPLOAD`).

## Caddy

`deploy/proxy/Caddyfile` est le fichier à placer dans `/etc/caddy/Caddyfile`.
Il est montré ici avec le nom d'hôte d'exemple et sans ses commentaires :

```
solstice.example.org {
	header Strict-Transport-Security max-age=31536000

	reverse_proxy 127.0.0.1:8080 {
		flush_interval -1
	}
}
```

- La première ligne est l'adresse du site. Remplacez le nom d'hôte du fichier
  par le vôtre. Caddy obtient et renouvelle le certificat correspondant.
- `header Strict-Transport-Security max-age=31536000` envoie HSTS pour un an.
- `reverse_proxy 127.0.0.1:8080` transmet à Solstice. Caddy envoie lui-même
  `X-Forwarded-Proto` et `X-Forwarded-For`.
- `flush_interval -1` envoie les données au navigateur dès que Solstice les
  écrit, ce dont les mises à jour en direct ont besoin.

Vérifiez le fichier, puis rechargez :

```sh
caddy validate --config /etc/caddy/Caddyfile
systemctl reload caddy
```

## nginx

`deploy/proxy/nginx.conf` va dans `/etc/nginx/sites-available/solstice`, lié
depuis `/etc/nginx/sites-enabled/`. Remplacez le nom d'hôte et les chemins du
certificat. Le fichier contient deux blocs `server`.

Le premier bloc écoute sur le port 80 et redirige tout vers HTTPS :

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name solstice.example.org;
    return 301 https://$host$request_uri;
}
```

Le second bloc sert le HTTPS :

- `ssl_certificate` et `ssl_certificate_key` désignent le certificat.
  L'exemple utilise l'arborescence `/etc/letsencrypt/live/<hôte>/`.
- `client_max_body_size 100m` autorise des envois de 100 MiB, le défaut de
  `SOLSTICE_HTTP_MAXUPLOAD`. Si vous en augmentez un, augmentez les deux.
- `add_header Strict-Transport-Security "max-age=31536000" always` envoie HSTS
  pour un an.
- `location /` transmet à `http://127.0.0.1:8080` avec `Host`,
  `X-Forwarded-Proto` (`$scheme`) et `X-Forwarded-For`
  (`$proxy_add_x_forwarded_for`).
- `location /api/events` transmet le flux des mises à jour en direct. Il
  utilise HTTP/1.1 avec un en-tête `Connection` vide, désactive
  `proxy_buffering` et `proxy_cache`, et fixe `proxy_read_timeout 1h` pour que
  nginx ne ferme pas le flux au bout de 60 secondes.

Vérifiez la configuration, puis rechargez :

```sh
nginx -t
systemctl reload nginx
```

## L'adresse publique

Indiquez à Solstice son adresse publique :

```sh
SOLSTICE_HTTP_BASEURL=https://solstice.example.org
```

L'assistant d'installation la demande et l'écrit dans le fichier de
configuration. Elle commence par `http://` ou `https://`, suivi d'un hôte et
d'un port facultatif, sans chemin.

Solstice l'utilise pour :

- les liens des e-mails. Sans elle, les e-mails partent sans liens, et les
  liens de réinitialisation de mot de passe et les invitations ne peuvent pas
  être envoyés ;
- les liens de l'API REST. Sans elle, ils sont construits à partir de l'en-tête
  `Host` de la requête et de `X-Forwarded-Proto`. Les charges utiles des
  webhooks omettent leurs liens `self` ;
- le cookie de session : avec une adresse `https://`, il est toujours
  `Secure`.

Un administrateur peut aussi changer l'adresse publique dans l'administration.
Une valeur enregistrée là l'emporte sur le fichier et sur l'environnement.

## HTTPS

Le HTTPS s'arrête au proxy. Entre le proxy et Solstice, le trafic est en HTTP
simple : ne laissez donc Solstice joignable que depuis le proxy.

- Sur la même machine, écoutez sur l'adresse de bouclage seulement :
  `SOLSTICE_HTTP_ADDR=127.0.0.1:8080`. Avec Docker, publiez le port sous la
  forme `"127.0.0.1:8080:8080"`.
- Sur une autre machine, limitez l'accès au proxy avec un pare-feu.

Cela compte aussi pour les proxies de confiance, ci-dessous.

## Proxies de confiance

Solstice compte les échecs de connexion par adresse de client et écrit cette
adresse dans le journal d'audit. Derrière un proxy, toutes les connexions
viennent du proxy : Solstice doit donc lire l'adresse du client dans
`X-Forwarded-For`.

Il ne le fait que pour les proxies que vous listez dans
`SOLSTICE_HTTP_TRUSTEDPROXIES`, sous forme d'adresses IP ou de plages CIDR
séparées par des virgules. Les deux exemples se connectent depuis
`127.0.0.1` :

```sh
SOLSTICE_HTTP_TRUSTEDPROXIES=127.0.0.1
```

Comment il lit l'en-tête :

- Si la connexion ne vient pas d'une adresse listée, l'en-tête est ignoré.
  Un client qui fixe lui-même `X-Forwarded-For` est quand même compté sous
  l'adresse d'où il s'est connecté.
- Si elle en vient, Solstice lit l'en-tête de droite à gauche, saute les
  adresses listées et prend la première autre adresse comme client. Les
  entrées qu'un client a écrites à gauche ne sont pas crues.
- Un en-tête absent, vide ou mal formé laisse l'adresse de la connexion.

Ne listez que des proxies que vous contrôlez, et assurez-vous que Solstice
n'est joignable que par eux. Sinon, laissez la liste vide. Une valeur que
Solstice ne sait pas lire l'arrête au démarrage, avec l'entrée citée dans
l'erreur.

### Sans proxy de confiance

Si la liste est vide derrière un proxy, Solstice voit l'adresse du proxy pour
tous les clients :

- Les échecs de connexion sont comptés pour cette seule adresse. Après cinq
  échecs depuis la même adresse, cette adresse est suspendue une minute, puis
  plus longtemps. Les tentatives ratées d'un seul visiteur touchent alors tous
  ceux qui passent par le proxy.
- Le journal d'audit montre l'adresse du proxy au lieu de celle du client.
- L'assistant d'installation limite les codes erronés par adresse de la même
  façon : tous ceux qui passent par le proxy partagent un seul compte.

Les sessions fonctionnent toujours, et le cookie reste `Secure` quand le proxy
envoie `X-Forwarded-Proto: https` ou que l'adresse publique est `https://`. La
liste des proxies de confiance change seulement l'adresse de client que
Solstice lit.

## Vérification

- `curl https://solstice.example.org/healthz` répond un corps JSON qui
  commence par `{"status":"ok"`.
- Ouvrez le même ticket dans deux navigateurs et modifiez-le dans l'un.
  L'autre se met à jour en moins d'une seconde. Sinon, le proxy met
  `/api/events` en mémoire tampon.
- Connectez-vous, puis regardez le cookie dans le navigateur : il est marqué
  `Secure`.
