BINARY  := bin/claude-watch
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build serve dev install dist release clean

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

serve: build
	$(BINARY) serve

dev: build
	$(BINARY) serve --no-browser

install: build
	cp $(BINARY) ~/.local/bin/claude-watch

# Cross-compile for all platforms into dist/
dist:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/claude-watch-darwin-amd64  .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/claude-watch-darwin-arm64  .
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/claude-watch-linux-amd64   .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/claude-watch-linux-arm64   .

# Build dist/ then create a GitHub release (install.sh included so the
# README curl URL always points to the latest release)
release: dist
	gh release create $(VERSION) \
		dist/claude-watch-darwin-amd64 \
		dist/claude-watch-darwin-arm64 \
		dist/claude-watch-linux-amd64  \
		dist/claude-watch-linux-arm64  \
		install.sh \
		--title "$(VERSION)" \
		--notes "Release $(VERSION)"

clean:
	rm -rf bin/ dist/
