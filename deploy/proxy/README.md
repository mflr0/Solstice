# Reverse proxy

Solstice serves plain HTTP. Put a reverse proxy in front of it for HTTPS. `nginx.conf` and `Caddyfile` are examples to copy and adapt; Caddy is the shorter one because it handles certificates itself.

Each example is explained in [docs/en/reverse-proxy.md](../../docs/en/reverse-proxy.md) ([français](../../docs/fr/reverse-proxy.md)).

## What the proxy must do

- Redirect plain HTTP to HTTPS.
- Pass the original `Host` header, and send `X-Forwarded-Proto` (https) and `X-Forwarded-For`, so Solstice marks its session cookie `Secure`.
- Send `Strict-Transport-Security` (HSTS) so browsers keep using HTTPS; both examples do, only on the HTTPS site (never on the plain HTTP redirect).
- Keep `GET /api/events` open without buffering and without closing it after 60 seconds.
- Accept request bodies up to 100 MB for attachments.
- Use `https://solstice.example.com` as the public address in `SOLSTICE_HTTP_BASEURL`.

## Checking

- `curl https://solstice.example.com/healthz` answers `{"status":"ok",...}`.
- Open the same issue in two browsers and change it in one; the other updates within a second. If it does not, the proxy buffers `/api/events`.

## Trusted proxies

Solstice counts failed sign-ins per client address, and writes that address to the audit log. Behind a proxy it sees the proxy's address for every client, so a single visitor's failed attempts would lock out everyone behind it.

List the proxies whose `X-Forwarded-For` header Solstice may believe in `SOLSTICE_HTTP_TRUSTEDPROXIES`, as IP addresses or CIDR ranges separated by commas. The examples forward the header from `127.0.0.1`, so:

```sh
SOLSTICE_HTTP_TRUSTEDPROXIES=127.0.0.1
```

Only addresses in the list are believed; a client that sets the header itself is still counted under the address it really connected from. Set this only for a proxy you control, make sure Solstice cannot be reached except through it, and leave it empty otherwise. A value Solstice cannot read stops it at start-up.
