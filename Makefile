# cx-onprem-orchestrator developer Makefile.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
LDFLAGS := -s -w -X github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/cli.Version=$(VERSION)
PKG := ./cmd/cx-onprem-orchestrator
BIN := bin/cx-onprem-orchestrator

# Release target matrix (os/arch).
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all build test vet tidy dist docker-slim docker-fat verify-pins clean

all: vet test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# Cross-compile every release target (proves zero-dependency static builds).
dist:
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
	  echo "building $$os/$$arch"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" \
	    -o dist/cx-onprem-orchestrator_$${os}_$${arch}$$ext $(PKG) || exit 1; \
	done
	@cd dist && sha256sum * > checksums.txt && echo "wrote dist/checksums.txt"

docker-slim:
	docker build -t cx-onprem-orchestrator:slim --build-arg VERSION=$(VERSION) -f Dockerfile .

docker-fat:
	docker build -t cx-onprem-orchestrator:fat --build-arg VERSION=$(VERSION) -f Dockerfile.fat .

# Re-download the pinned engine tools, verify their digests/versions against
# internal/resolve/manifest.lock, and run the golden exit-code tests. Gate every
# version bump through this so a silent upstream flag/exit-code change is caught.
verify-pins:
	@echo "Verifying pinned docker image digests against manifest.lock..."
	go run ./hack/verify-pins
	@echo "Running golden exit-code / threshold / scanner tests..."
	go test ./internal/exit/... ./internal/threshold/... ./internal/scanner/...

clean:
	rm -rf bin dist
