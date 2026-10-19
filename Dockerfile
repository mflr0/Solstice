# Solstice: the web interface is built first, then embedded into a single
# static Go binary, shipped on a minimal image that runs as an unprivileged
# user.
#
#   docker build -t solstice --build-arg VERSION=1.0.0 .
#
# See deploy/docker/ for a ready-to-run setup with MariaDB.

FROM node:24-alpine@sha256:44fce696f38ba8c987c4db89954051000bbd05ffd8d8659ba15ed4a8602e937f AS ui
WORKDIR /src/frontend
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
# The packageManager field of package.json owns the pnpm version.
RUN npm install --global "$(node -p 'require("./package.json").packageManager')"
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
# Type-checks, then writes ../backend/internal/web/dist.
RUN pnpm build

FROM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS server
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=ui /src/backend/internal/web/dist ./internal/web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/solstice ./cmd/solstice \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=server /out/solstice /usr/local/bin/solstice
# The data directory belongs to the unprivileged user, so a fresh volume does too.
COPY --from=server --chown=65532:65532 /out/data /var/lib/solstice
# SOLSTICE_INSTALL: the update page shows how to update a container (rebuild
# the image) instead of offering to update in place. JSON logs suit log
# collectors.
ENV SOLSTICE_DATA=/var/lib/solstice \
    SOLSTICE_CONFIG=/var/lib/solstice/solstice.conf \
    SOLSTICE_HTTP_ADDR=:8080 \
    SOLSTICE_INSTALL=docker \
    SOLSTICE_LOG_FORMAT=json
VOLUME /var/lib/solstice
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=6s --start-period=90s --retries=3 \
  CMD ["/usr/local/bin/solstice", "health"]
ENTRYPOINT ["/usr/local/bin/solstice"]
CMD ["serve"]
