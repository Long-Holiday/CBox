.PHONY: all build test fmt vet clean install

all: build

build:
	mkdir -p bin
	go build -o bin/cbox ./cmd/cbox
	go build -o bin/cboxd ./cmd/cboxd

test:
	go test -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf bin

install:
	go install ./cmd/cbox
	go install ./cmd/cboxd
