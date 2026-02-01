#!/bin/bash
#
# Generate Go protobuf code from Meshtastic .proto files
#
# Usage: ./scripts/gen-proto.sh [--from-github]
#
# Options:
#   --from-github  Clone/update protobufs from github.com/meshtastic/protobufs
#
# Prerequisites:
#   - protoc (Protocol Buffers compiler)
#   - protoc-gen-go (install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest)
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$PROJECT_ROOT/proto/meshtastic"
PB_DIR="$PROJECT_ROOT/pkg/pb"
GITHUB_REPO="https://github.com/meshtastic/protobufs.git"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    local missing=0

    if ! command -v protoc &> /dev/null; then
        log_error "protoc is not installed. Install Protocol Buffers compiler."
        log_info "  macOS: brew install protobuf"
        log_info "  Ubuntu: apt-get install protobuf-compiler"
        missing=1
    fi

    if ! command -v protoc-gen-go &> /dev/null; then
        log_error "protoc-gen-go is not installed."
        log_info "  Install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
        missing=1
    fi

    if [ $missing -eq 1 ]; then
        exit 1
    fi
}

clone_or_update_protos() {
    if [ -d "$PROTO_DIR/.git" ]; then
        log_info "Updating protobufs from GitHub..."
        cd "$PROTO_DIR"
        git pull origin master
        cd "$PROJECT_ROOT"
    elif [ -d "$PROTO_DIR" ] && [ -n "$(ls -A "$PROTO_DIR"/*.proto 2>/dev/null)" ]; then
        log_info "Proto directory exists with .proto files, using existing files."
    else
        log_info "Cloning protobufs from GitHub..."
        mkdir -p "$(dirname "$PROTO_DIR")"
        git clone "$GITHUB_REPO" "$PROTO_DIR"
    fi
}

generate_protos() {
    log_info "Generating Go code from .proto files..."

    mkdir -p "$PB_DIR"

    # List of key proto files to generate
    PROTO_FILES=(
        "meshtastic/mesh.proto"
        "meshtastic/portnums.proto"
        "meshtastic/telemetry.proto"
        "meshtastic/config.proto"
        "meshtastic/module_config.proto"
        "meshtastic/channel.proto"
        "meshtastic/admin.proto"
        "meshtastic/apponly.proto"
        "meshtastic/deviceonly.proto"
        "meshtastic/localonly.proto"
        "meshtastic/clientonly.proto"
        "meshtastic/remote_hardware.proto"
        "meshtastic/storeforward.proto"
        "meshtastic/connection_status.proto"
    )

    # Check which files exist
    EXISTING_FILES=()
    for proto in "${PROTO_FILES[@]}"; do
        if [ -f "$PROJECT_ROOT/proto/$proto" ]; then
            EXISTING_FILES+=("$PROJECT_ROOT/proto/$proto")
        fi
    done

    if [ ${#EXISTING_FILES[@]} -eq 0 ]; then
        log_warn "No .proto files found in $PROTO_DIR"
        log_info "Using hand-written Go structs in pkg/pb/"
        return 0
    fi

    log_info "Found ${#EXISTING_FILES[@]} proto files to process"

    # Generate Go code
    protoc -I="$PROJECT_ROOT/proto" \
        --go_out="$PB_DIR" \
        --go_opt=paths=source_relative \
        --go_opt=Mmeshtastic/mesh.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
        --go_opt=Mmeshtastic/portnums.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
        --go_opt=Mmeshtastic/telemetry.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
        --go_opt=Mmeshtastic/config.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
        --go_opt=Mmeshtastic/module_config.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
        --go_opt=Mmeshtastic/channel.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
        --go_opt=Mmeshtastic/admin.proto=github.com/meshtastic/meshtastic-go/pkg/pb \
        "${EXISTING_FILES[@]}" 2>&1 || true

    log_info "Proto generation complete!"
}

main() {
    cd "$PROJECT_ROOT"

    check_prerequisites

    if [ "$1" == "--from-github" ]; then
        clone_or_update_protos
    fi

    if [ -d "$PROTO_DIR" ] && [ -n "$(ls -A "$PROTO_DIR"/*.proto 2>/dev/null)" ]; then
        generate_protos
    else
        log_warn "No proto files found. Options:"
        log_info "  1. Run: $0 --from-github"
        log_info "  2. Run: make proto-init"
        log_info "  3. Use the hand-written Go structs in pkg/pb/"
    fi
}

main "$@"
