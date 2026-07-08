.PHONY: all build test clean lint release

APP := wg-bot

all: fmt lint test build

build:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(APP)-amd64 ./cmd/$(APP)/
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags="-s -w" -o $(APP)-mipsle ./cmd/$(APP)/

test:
	go test -v -count=1 ./...

lint:
	golangci-lint run ./... 2>/dev/null || go vet ./...

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
