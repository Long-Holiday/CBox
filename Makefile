.PHONY: all build build-static package test fmt vet clean install

VERSION ?= 0.1.0
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS := -s -w -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)

all: build

build:
	mkdir -p bin
	go build -ldflags="$(LDFLAGS)" -o bin/cbox ./cmd/cbox
	go build -ldflags="$(LDFLAGS)" -o bin/cboxd ./cmd/cboxd

build-static:
	mkdir -p bin dist
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/cbox ./cmd/cbox
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/cboxd ./cmd/cboxd
	cp bin/cbox dist/cbox-linux-amd64

package: build-static
	mkdir -p dist
	tar -czvf dist/cbox_$(VERSION)_linux_amd64.tar.gz -C bin cbox cboxd
	@echo "Single-binary executable package created at dist/cbox_$(VERSION)_linux_amd64.tar.gz"
	@echo "Stand-alone static executable created at dist/cbox-linux-amd64"

test:
	go test -count=1 ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf bin dist

install:
	go install -ldflags="$(LDFLAGS)" ./cmd/cbox
	go install -ldflags="$(LDFLAGS)" ./cmd/cboxd
