.PHONY: all build clean proto proto-gen proto-install test lint deps run

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
PROTO_DIR := proto/meshtastic
PB_DIR := pkg/pb

all: build

# Generate protobufs from .proto files (if proto submodule is initialized)
proto-gen:
	@echo "Generating protobuf code from .proto files..."
	@mkdir -p $(PB_DIR)
	@if [ -d "$(PROTO_DIR)" ] && [ -n "$$(ls -A $(PROTO_DIR)/*.proto 2>/dev/null)" ]; then \
		protoc -I=proto \
			--go_out=$(PB_DIR) \
			--go_opt=paths=source_relative \
			--go_opt=Mmeshtastic/mesh.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
			--go_opt=Mmeshtastic/portnums.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
			--go_opt=Mmeshtastic/telemetry.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
			--go_opt=Mmeshtastic/config.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
			--go_opt=Mmeshtastic/module_config.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
			--go_opt=Mmeshtastic/channel.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
			--go_opt=Mmeshtastic/admin.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
			$(PROTO_DIR)/*.proto; \
		echo "Protobuf generation complete."; \
	else \
		echo "No .proto files found. Using hand-written Go structs in pkg/pb/"; \
		echo "Run 'make proto-init' to clone the official protobufs."; \
	fi

# Install protoc plugins
proto-install:
	@echo "Installing protoc-gen-go..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@echo "Installing protoc-gen-go-grpc..."
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Alias for backward compatibility
proto: proto-gen

build:
	@echo "Building meshtastic-go..."
	go build $(LDFLAGS) -o bin/meshtastic ./cmd/meshtastic

build-all:
	@echo "Building for all platforms..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/meshtastic-linux-amd64 ./cmd/meshtastic
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/meshtastic-linux-arm64 ./cmd/meshtastic
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/meshtastic-darwin-amd64 ./cmd/meshtastic
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/meshtastic-darwin-arm64 ./cmd/meshtastic
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/meshtastic-windows-amd64.exe ./cmd/meshtastic

clean:
	rm -rf bin/

test:
	go test -v -race ./...

test-cover:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

deps:
	go mod download
	go mod verify

tidy:
	go mod tidy

run:
	go run ./cmd/meshtastic

run-dev:
	go run ./cmd/meshtastic --config config.yaml

# Clone protobuf definitions
proto-init:
	git submodule add https://github.com/meshtastic/protobufs.git proto/meshtastic || true
	git submodule update --init --recursive

proto-update:
	git submodule update --remote proto/meshtastic

# Docker
docker-build:
	docker build -t meshtastic-go .

docker-run:
	docker run -p 8080:8080 -v $(PWD)/data:/app/data meshtastic-go
