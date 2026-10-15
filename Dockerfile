# Solstice: the web interface is built first, then embedded into a single
# static Go binary, shipped on a minimal image that runs as an unprivileged
# user.
#
#   docker build -t solstice --build-arg VERSION=1.0.0 .
#
# See deploy/docker/ for a ready-to-run setup with MariaDB.

FROM node:24-alpine AS ui
WORKDIR /src/frontend
RUN npm install --global pnpm@12.11.2
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
# Type-checks, then writes ../backend/internal/web/dist.
RUN pnpm build

FROM golang:1.27-alpine AS server
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=ui /src/backend/internal/web/dist ./internal/web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/solstice ./cmd/solstice \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/solstice /usr/local/bin/solstice
# The data directory belongs to the unprivileged user, so a fresh volume does too.
COPY --from=server --chown=65532:65532 /out/data /var/lib/solstice
# SOLSTICE_INSTALL: the update page shows how to update a container (rebuild
# the image) instead of offering to update in place.
ENV SOLSTICE_DATA=/var/lib/solstice \
    SOLSTICE_CONFIG=/var/lib/solstice/solstice.conf \
    SOLSTICE_ADDR=:8080 \
    SOLSTICE_INSTALL=docker
VOLUME /var/lib/solstice
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=6s --start-period=90s --retries=3 \
  CMD ["/usr/local/bin/solstice", "health"]
ENTRYPOINT ["/usr/local/bin/solstice"]
CMD ["serve"]
