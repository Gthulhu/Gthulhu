#!/bin/bash
# Build script for Gthulhu container images

set -e

# Configuration
REGISTRY="${REGISTRY:-localhost:5000}"
TAG="${TAG:-latest}"
BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're in the right directory
if [ ! -f "../../Dockerfile" ] || [ ! -f "../../api/Dockerfile" ]; then
    echo_error "This script should be run from the chart/gthulhu directory"
    echo_error "Expected structure: Gthulhu/chart/gthulhu/build-images.sh"
    exit 1
fi

# Build scheduler image
build_scheduler() {
    echo_info "Building Gthulhu scheduler image..."
    
    cd ../../
    
    # Build the main binary first
    if [ ! -f "main" ]; then
        echo_info "Building Gthulhu scheduler binary..."
        make build || {
            echo_error "Failed to build scheduler binary"
            exit 1
        }
    fi
    
    # Build Docker image
    docker build \
        --build-arg BUILD_DATE="${BUILD_DATE}" \
        --build-arg GIT_COMMIT="${GIT_COMMIT}" \
        -t "${REGISTRY}/gthulhu:${TAG}" \
        -f Dockerfile .
    
    echo_info "Scheduler image built: ${REGISTRY}/gthulhu:${TAG}"
    cd chart/gthulhu
}

# Build API server image
build_api() {
    echo_info "Building Gthulhu API server image..."
    
    cd ../../api
    
    # Build the API binary first
    echo_info "Building API server binary..."
    go build -o main . || {
        echo_error "Failed to build API binary"
        exit 1
    }
    
    # Build Docker image
    docker build \
        --build-arg BUILD_DATE="${BUILD_DATE}" \
        --build-arg GIT_COMMIT="${GIT_COMMIT}" \
        -t "${REGISTRY}/gthulhu-api:${TAG}" \
        -f Dockerfile .
    
    echo_info "API image built: ${REGISTRY}/gthulhu-api:${TAG}"
    cd ../chart/gthulhu
}

# Push images to registry
push_images() {
    if [ "$1" = "--push" ]; then
        echo_info "Pushing images to registry..."
        docker push "${REGISTRY}/gthulhu:${TAG}"
        docker push "${REGISTRY}/gthulhu-api:${TAG}"
        echo_info "Images pushed successfully"
    fi
}

# Show image info
show_images() {
    echo_info "Built images:"
    docker images | grep -E "gthulhu|REPOSITORY" || echo "No images found"
}

# Main execution
main() {
    case "${1:-all}" in
        "scheduler")
            build_scheduler
            ;;
        "api")
            build_api
            ;;
        "all")
            build_scheduler
            build_api
            ;;
        "--help"|"-h")
            echo "Usage: $0 [scheduler|api|all] [--push]"
            echo ""
            echo "Options:"
            echo "  scheduler    Build only the scheduler image"
            echo "  api         Build only the API server image"
            echo "  all         Build both images (default)"
            echo "  --push      Push images to registry after building"
            echo ""
            echo "Environment variables:"
            echo "  REGISTRY    Container registry (default: localhost:5000)"
            echo "  TAG         Image tag (default: latest)"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Build both images"
            echo "  $0 scheduler                          # Build only scheduler"
            echo "  $0 all --push                         # Build and push both images"
            echo "  REGISTRY=docker.io/myorg TAG=v1.0.0 $0 all --push"
            exit 0
            ;;
        *)
            echo_error "Unknown option: $1"
            echo "Use $0 --help for usage information"
            exit 1
            ;;
    esac
    
    push_images "$2"
    show_images
    
    echo_info "Build completed successfully!"
    echo_info "Images are ready for use with Helm chart"
    echo ""
    echo_info "To use these images with Helm:"
    echo "  helm install gthulhu . \\"
    echo "    --set scheduler.image.repository=${REGISTRY}/gthulhu \\"
    echo "    --set scheduler.image.tag=${TAG} \\"
    echo "    --set api.image.repository=${REGISTRY}/gthulhu-api \\"
    echo "    --set api.image.tag=${TAG}"
}

main "$@"
