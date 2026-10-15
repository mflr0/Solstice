# Reverse proxy

Solstice serves plain HTTP. Put a reverse proxy in front of it for HTTPS. `nginx.conf` and `Caddyfile` are examples to copy and adapt; Caddy is the shorter one because it handles certificates itself.

## What the proxy must do

- Redirect plain HTTP to HTTPS.
- Pass the original `Host` header, and send `X-Forwarded-Proto` (https) and `X-Forwarded-For`, so Solstice marks its session cookie `Secure`.
- Keep `GET /api/events` open without buffering and without closing it after 60 seconds.
- Accept request bodies up to 100 MB for attachments.
- Use `https://solstice.example.com` as the public address in `SOLSTICE_BASE_URL`.

## Checking

- `curl https://solstice.example.com/healthz` answers `{"status":"ok",...}`.
- Open the same issue in two browsers and change it in one; the other updates within a second. If it does not, the proxy buffers `/api/events`.

## Known limitation

Solstice counts failed sign-ins per client address, and behind a proxy it sees the proxy's address for every client.
