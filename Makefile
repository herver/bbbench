.PHONY: all clean test

# Default target: static build with musl
all: bbbench check-config

bbbench:
	CGO_ENABLED=1 CC=x86_64-linux-musl-gcc go build -ldflags="-linkmode external -extldflags '-static'" ./cmd/bbbench

check-config:
	CGO_ENABLED=1 CC=x86_64-linux-musl-gcc go build -ldflags="-linkmode external -extldflags '-static'" ./cmd/check-config

clean:
	rm -f bbbench

test:
	go test ./...
