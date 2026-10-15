# Common tasks. Needs Go and Node with pnpm; Docker for the end-to-end tests
# and the image.
VERSION ?= dev

.PHONY: build test lint e2e docker release

# The single binary, interface embedded → backend/solstice
build:
	cd frontend && pnpm install --frozen-lockfile && pnpm build
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o solstice ./cmd/solstice

test:
	cd backend && go test ./...
	cd frontend && pnpm test

lint:
	cd backend && test -z "$$(gofmt -l .)" && go vet ./...
	cd frontend && pnpm lint

# Imports a synthetic Jira on PostgreSQL and MySQL, compares, runs every test.
e2e:
	./scripts/e2e-import.sh

docker:
	docker build -t solstice:$(VERSION) --build-arg VERSION=$(VERSION) .

release:
	./scripts/release.sh $(VERSION)
