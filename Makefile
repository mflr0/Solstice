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

# The static checks of .github/workflows/ci.yml; keep the two in step.
lint:
	cd backend && test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	cd backend && go vet ./...
	cd backend && go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
	cd backend && go run github.com/securego/gosec/v2/cmd/gosec@v2.29.0 -quiet -exclude=G104 -exclude-dir=cmd/jira-fixture -exclude-dir=internal/jirafixture ./...
	cd frontend && pnpm install --frozen-lockfile && pnpm format:check && pnpm lint

# Imports a synthetic Jira on PostgreSQL and MySQL, compares, runs every test.
e2e:
	./scripts/e2e-import.sh

docker:
	docker build -t solstice:$(VERSION) --build-arg VERSION=$(VERSION) .

release:
	./scripts/release.sh $(VERSION)
