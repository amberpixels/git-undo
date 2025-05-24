#!/bin/bash
set -euo pipefail

# Script to run integration tests locally using Docker
# This provides the same isolated environment as CI

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Source shared colors and logging functions
source "$SCRIPT_DIR/colors.sh"

# Parse command line arguments
MODE="dev"  # Default to dev mode for local testing
DOCKERFILE="scripts/integration/Dockerfile.dev"
DESCRIPTION="current local changes"

while [[ $# -gt 0 ]]; do
    case $1 in
        --prod|--production)
            MODE="production"
            DOCKERFILE="scripts/integration/Dockerfile"
            DESCRIPTION="real user experience (published releases)"
            shift
            ;;
        --dev|--development)
            MODE="dev"
            DOCKERFILE="scripts/integration/Dockerfile.dev"
            DESCRIPTION="current local changes"
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--prod|--dev]"
            echo ""
            echo "Options:"
            echo "  --prod, --production   Test real user experience (downloads from GitHub)"
            echo "  --dev, --development   Test current local changes (default)"
            echo "  --help, -h            Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Check if Docker is available
if ! command -v docker >/dev/null 2>&1; then
    log_error "Docker is not installed or not in PATH"
    log_error "Please install Docker to run integration tests"
    exit 1
fi

# Check if Docker daemon is running
if ! docker info >/dev/null 2>&1; then
    log_error "Docker daemon is not running"
    log_error "Please start Docker daemon and try again"
    exit 1
fi

log_info "🧪 Integration test mode: $MODE"
log_info "📝 Testing: $DESCRIPTION"
log_info "🐳 Using dockerfile: $DOCKERFILE"
log_info "📂 Project root: $PROJECT_ROOT"
echo ""

# Build the integration test image
IMAGE_NAME="git-undo-integration:local-$MODE"
log_info "Building Docker image: $IMAGE_NAME"

cd "$PROJECT_ROOT"

if docker build -f "$DOCKERFILE" -t "$IMAGE_NAME" .; then
    log_success "Docker image built successfully"
else
    log_error "Failed to build Docker image"
    exit 1
fi

# Run the integration tests in the container
log_info "Running integration tests in isolated container..."

if docker run --rm "$IMAGE_NAME"; then
    log_success "Integration tests completed successfully!"
    log_info "All tests passed in isolated environment"
    exit 0
else
    log_error "Integration tests failed!"
    log_error "Check the output above for details"
    exit 1
fi 