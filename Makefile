.PHONY: all build test clean lint security release

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
	gosec -quiet ./...
	gitleaks detect --no-git --verbose

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
