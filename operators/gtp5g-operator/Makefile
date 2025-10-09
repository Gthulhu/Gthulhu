# Image URL to use all building/pushing image targets
IMG ?= localhost:5000/gtp5g-operator:latest
INSTALLER_IMG ?= localhost:5000/gtp5g-installer:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: ## Generate CRD manifests
	@echo "CRD manifests already in config/crd/"

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary
	go build -o bin/manager main.go

.PHONY: run
run: fmt vet ## Run from your host
	go run ./main.go

.PHONY: docker-build
docker-build: ## Build docker image for operator
	docker build -t ${IMG} -f Dockerfile .

.PHONY: docker-push
docker-push: ## Push docker image for operator
	docker push ${IMG}

.PHONY: docker-build-installer
docker-build-installer: ## Build docker image for installer
	cd installer && docker build -t ${INSTALLER_IMG} .

.PHONY: docker-push-installer
docker-push-installer: ## Push docker image for installer
	docker push ${INSTALLER_IMG}

.PHONY: docker-build-all
docker-build-all: docker-build docker-build-installer ## Build all docker images

.PHONY: docker-push-all
docker-push-all: docker-push docker-push-installer ## Push all docker images

##@ Deployment

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config
	kubectl apply -f config/crd/gtp5gmodule.yaml

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config
	kubectl delete -f config/crd/gtp5gmodule.yaml

.PHONY: deploy-sample
deploy-sample: ## Deploy sample GTP5GModule
	kubectl apply -f config/samples/gtp5gmodule_sample.yaml

.PHONY: undeploy-sample
undeploy-sample: ## Undeploy sample GTP5GModule
	kubectl delete -f config/samples/gtp5gmodule_sample.yaml
