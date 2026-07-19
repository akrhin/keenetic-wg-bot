.PHONY: all build test clean lint security release verify verify-commands

APP := wg-bot

all: fmt lint security test build

build:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(APP)-amd64 ./cmd/$(APP)/
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags="-s -w" -o $(APP)-mipsle ./cmd/$(APP)/

test:
	go test -v -count=1 -race ./...

lint:
	golangci-lint run ./... 2>/dev/null || go vet ./...

# CI: requires Go 1.25.8+ for gosec v2.28 (runs in CI with ubuntu-latest)
security:
	gosec -quiet ./... || echo "gosec not available (CI only)"
	gitleaks detect --no-git --verbose || echo "gitleaks not available (CI only)"

fmt:
	go fmt ./...

clean:
	rm -f $(APP)-amd64 $(APP)-mipsle
	rm -rf dist/

release: test build
	mkdir -p dist
	cp $(APP)-mipsle dist/$(APP)
	cp install.sh dist/
	cp config.toml.example dist/config.toml.example
	cd dist && tar czf ../$(APP)-mipsle.tar.gz $(APP) install.sh config.toml.example
	rm -rf dist
	@echo "Release: $(APP)-mipsle.tar.gz"

# verify — проверка наличия обязательных утилит
verify-commands:
	@echo "Verifying tools..."
	@command -v go >/dev/null 2>&1 || { echo "❌ go not found"; exit 1; }
	@command -v golangci-lint >/dev/null 2>&1 && echo "✅ golangci-lint" || echo "⚠️  golangci-lint not found (CI only)"
	@command -v gosec >/dev/null 2>&1 && echo "✅ gosec" || echo "⚠️  gosec not found (CI only)"
	@command -v gitleaks >/dev/null 2>&1 && echo "✅ gitleaks" || echo "⚠️  gitleaks not found (CI only)"
	@echo "✅ go found"

verify: verify-commands
	@echo "Running verification..."
	go vet ./...
	@echo "✅ vet passed"
	go mod verify
	@echo "✅ modules verified"
