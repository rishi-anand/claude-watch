BINARY  := bin/claude-watch
VERSION ?= dev

.PHONY: build serve dev install clean

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) .

serve: build
	$(BINARY) serve

dev: build
	$(BINARY) serve --no-browser

install: build
	cp $(BINARY) ~/.local/bin/claude-watch

clean:
	rm -rf bin/ dist/
