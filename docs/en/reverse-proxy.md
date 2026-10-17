# Reverse proxy

Solstice listens in plain HTTP and has no setting for certificates. Put a
reverse proxy in front of it to serve HTTPS. Related pages:
[installation](install.md), [backups](backups.md), [upgrades](upgrades.md).

The repository has two examples to copy and adapt: `deploy/proxy/Caddyfile`
and `deploy/proxy/nginx.conf`. Caddy is the shorter one because it gets and
renews the certificate by itself.

## What the proxy must do

- Redirect plain HTTP to HTTPS.
- Pass the original `Host` header.
- Send `X-Forwarded-Proto` (`https`) and `X-Forwarded-For`. Solstice marks its
  session cookie `Secure` from `X-Forwarded-Proto: https` only when the proxy
  is listed in `SOLSTICE_HTTP_TRUSTEDPROXIES` (see below), or when the public
  address is `https://`.
- Send `Strict-Transport-Security` (HSTS) on the HTTPS site, so that browsers
  keep using HTTPS. Never send it on the plain HTTP redirect.
- Keep `GET /api/events` open, without buffering and without closing it after
  60 seconds. This is the stream of live updates.
- Accept request bodies up to the size of an upload, 100 MiB by default (see
  `SOLSTICE_HTTP_MAXUPLOAD`).

## Caddy

`deploy/proxy/Caddyfile` is the file to put in `/etc/caddy/Caddyfile`. It
is shown here with the example host name and without its comments:

```
solstice.example.org {
	header Strict-Transport-Security max-age=31536000

	reverse_proxy 127.0.0.1:8080 {
		flush_interval -1
	}
}
```

- The first line is the site address. Replace the host name in the file with
  yours. Caddy gets and renews the certificate for it.
- `header Strict-Transport-Security max-age=31536000` sends HSTS for one year.
- `reverse_proxy 127.0.0.1:8080` forwards to Solstice. Caddy sends
  `X-Forwarded-Proto` and `X-Forwarded-For` by itself.
- `flush_interval -1` sends data to the browser as soon as Solstice writes it,
  which live updates need.

Check the file, then reload:

```sh
caddy validate --config /etc/caddy/Caddyfile
systemctl reload caddy
```

## nginx

`deploy/proxy/nginx.conf` goes in `/etc/nginx/sites-available/solstice`, linked
from `/etc/nginx/sites-enabled/`. Replace the host name and the certificate
paths. The file has two `server` blocks.

The first block listens on port 80 and redirects everything to HTTPS:

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name solstice.example.org;
    return 301 https://$host$request_uri;
}
```

The second block serves HTTPS:

- `ssl_certificate` and `ssl_certificate_key` point to the certificate. The
  example uses the `/etc/letsencrypt/live/<host>/` layout.
- `client_max_body_size 100m` allows uploads of 100 MiB, the default of
  `SOLSTICE_HTTP_MAXUPLOAD`. Raise both together if you raise one.
- `add_header Strict-Transport-Security "max-age=31536000" always` sends HSTS
  for one year.
- `location /` forwards to `http://127.0.0.1:8080` with `Host`,
  `X-Forwarded-Proto` (`$scheme`) and `X-Forwarded-For`
  (`$proxy_add_x_forwarded_for`).
- `location /api/events` forwards the live update stream. It uses HTTP/1.1
  with an empty `Connection` header, turns off `proxy_buffering` and
  `proxy_cache`, and sets `proxy_read_timeout 1h` so that nginx does not close
  the stream after 60 seconds.

Check the configuration, then reload:

```sh
nginx -t
systemctl reload nginx
```

## The public address

Tell Solstice its public address:

```sh
SOLSTICE_HTTP_BASEURL=https://solstice.example.org
```

The setup wizard asks for it and writes it to the configuration file. It is
`http://` or `https://`, a host and an optional port, with no path.

Solstice uses it for:

- links in e-mails. Without it, e-mails go out without links, and password
  reset links and invitations cannot be sent;
- the links of the REST API. Without it, they are built from the request's
  `Host` header and `X-Forwarded-Proto`. Webhook payloads leave their `self`
  links out;
- the session cookie: with an `https://` address it is always `Secure`.

An administrator can also change the public address in the administration. A
value saved there wins over the file and the environment.

## HTTPS

HTTPS ends at the proxy. Between the proxy and Solstice the traffic is plain
HTTP, so keep Solstice reachable only from the proxy:

- On the same machine, listen on the loopback address only:
  `SOLSTICE_HTTP_ADDR=127.0.0.1:8080`. In Docker, publish the port as
  `"127.0.0.1:8080:8080"`.
- On another machine, restrict access to the proxy with a firewall.

This also matters for trusted proxies, below.

## Trusted proxies

Solstice counts failed sign-ins per client address and writes that address to
the audit log. Behind a proxy, every connection comes from the proxy, so
Solstice must read the client's address from `X-Forwarded-For`.

It does so only for proxies you list in `SOLSTICE_HTTP_TRUSTEDPROXIES`, as IP
addresses or CIDR ranges separated by commas. Both examples connect from
`127.0.0.1`:

```sh
SOLSTICE_HTTP_TRUSTEDPROXIES=127.0.0.1
```

How it reads the header:

- If the connection does not come from a listed address, the header is
  ignored. A client that sets `X-Forwarded-For` itself is still counted under
  the address it connected from.
- If it does, Solstice reads the header from right to left, skips the listed
  addresses, and takes the first other address as the client. Entries a client
  wrote on the left are not believed.
- A missing, empty or malformed header leaves the address of the connection.

List only proxies you control, and make sure Solstice cannot be reached except
through them. Otherwise leave the list empty. A value Solstice cannot read
stops it at start, with the entry quoted in the error.

### Without a trusted proxy

If the list is empty behind a proxy, Solstice sees the proxy's address for
every client:

- Failed sign-ins are counted for that one address. After five failures from
  the same address, that address is suspended for a minute, then longer. One
  visitor's failed attempts then affect everyone behind the proxy.
- The audit log shows the proxy's address instead of the client's.
- The setup wizard throttles wrong codes per address in the same way, so
  everyone behind the proxy shares one count.

Sessions still work. The cookie is `Secure` only if the public address is
`https://`, because Solstice ignores `X-Forwarded-Proto` from a proxy that is
not listed.

## Checking

- `curl https://solstice.example.org/healthz` answers a JSON body that starts
  with `{"status":"ok"`.
- Open the same issue in two browsers and change it in one. The other updates
  within a second. If it does not, the proxy buffers `/api/events`.
- Sign in, then look at the cookie in the browser: it is marked `Secure`.
