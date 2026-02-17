.PHONY: all clean test

# Default target: static build with musl
all: bbbench

bbbench:
	CGO_ENABLED=1 CC=x86_64-linux-musl-gcc go build -ldflags="-linkmode external -extldflags '-static'" ./cmd/bbbench

clean:
	rm -f bbbench

test:
	go test ./...
