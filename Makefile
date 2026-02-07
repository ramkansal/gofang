BINARY   = gofang
VERSION  = 1.0.0
MODULE   = github.com/ramkansal/gofang
CMD      = ./cmd/gofang

LDFLAGS  = -s -w -X main.version=$(VERSION)

# Default: build for current OS/arch
.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

# Build all platforms
.PHONY: all
all: clean build-linux build-windows build-darwin build-freebsd

# Linux
.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 $(CMD)
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 $(CMD)

# Windows
.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe $(CMD)
	GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-arm64.exe $(CMD)

# macOS
.PHONY: build-darwin
build-darwin:
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 $(CMD)
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 $(CMD)

# FreeBSD
.PHONY: build-freebsd
build-freebsd:
	GOOS=freebsd GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-freebsd-amd64 $(CMD)

.PHONY: clean
clean:
	rm -rf dist/
	rm -f $(BINARY) $(BINARY).exe

.PHONY: test
test:
	go test ./...

.PHONY: tidy
tidy:
	go mod tidy
