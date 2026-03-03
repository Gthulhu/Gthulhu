# Makefile for Gthulhu Helm Chart

CHART_NAME := gthulhu
NAMESPACE := default
RELEASE_NAME := gthulhu

# Default values
VALUES_FILE := values.yaml

# Environment specific values
PROD_VALUES := values-production.yaml
DEV_VALUES := values-development.yaml
TEST_VALUES := values-testing.yaml

.PHONY: help lint test install install-dev install-prod install-test upgrade uninstall clean

help: ## Show this help message
	@echo "Gthulhu Helm Chart Management"
	@echo "============================="
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

lint: ## Lint the Helm chart
	@echo "🔍 Linting Helm chart..."
	helm lint $(CHART_NAME)

test: lint ## Test the Helm chart (dry-run)
	@echo "🧪 Testing Helm chart..."
	helm install $(RELEASE_NAME)-test $(CHART_NAME) --dry-run --debug

template: ## Generate Kubernetes manifests from the chart
	@echo "📄 Generating templates..."
	helm template $(RELEASE_NAME) $(CHART_NAME) --values $(VALUES_FILE)

install: lint ## Install with default values
	@echo "🚀 Installing Gthulhu with default values..."
	helm install $(RELEASE_NAME) $(CHART_NAME) --namespace $(NAMESPACE) --create-namespace

install-dev: lint ## Install for development environment
	@echo "🛠️  Installing Gthulhu for development..."
	helm install $(RELEASE_NAME)-dev $(CHART_NAME) \
		--values $(DEV_VALUES) \
		--namespace $(NAMESPACE)-dev \
		--create-namespace

install-prod: lint ## Install for production environment
	@echo "🏭 Installing Gthulhu for production..."
	helm install $(RELEASE_NAME) $(CHART_NAME) \
		--values $(PROD_VALUES) \
		--namespace $(NAMESPACE) \
		--create-namespace

install-test: lint ## Install for testing environment
	@echo "🧪 Installing Gthulhu for testing..."
	helm install $(RELEASE_NAME)-test $(CHART_NAME) \
		--values $(TEST_VALUES) \
		--namespace $(NAMESPACE)-test \
		--create-namespace

upgrade: lint ## Upgrade existing installation
	@echo "⬆️  Upgrading Gthulhu..."
	helm upgrade $(RELEASE_NAME) $(CHART_NAME) --namespace $(NAMESPACE)

upgrade-dev: lint ## Upgrade development installation
	@echo "⬆️  Upgrading Gthulhu development..."
	helm upgrade $(RELEASE_NAME)-dev $(CHART_NAME) \
		--values $(DEV_VALUES) \
		--namespace $(NAMESPACE)-dev

upgrade-prod: lint ## Upgrade production installation
	@echo "⬆️  Upgrading Gthulhu production..."
	helm upgrade $(RELEASE_NAME) $(CHART_NAME) \
		--values $(PROD_VALUES) \
		--namespace $(NAMESPACE)

status: ## Show installation status
	@echo "📊 Checking Gthulhu status..."
	helm status $(RELEASE_NAME) --namespace $(NAMESPACE) || true
	@echo ""
	@echo "Development:"
	helm status $(RELEASE_NAME)-dev --namespace $(NAMESPACE)-dev || true
	@echo ""
	@echo "Testing:"
	helm status $(RELEASE_NAME)-test --namespace $(NAMESPACE)-test || true

list: ## List all Gthulhu installations
	@echo "📋 Listing all Gthulhu installations..."
	helm list -A | grep gthulhu || echo "No Gthulhu installations found"

uninstall: ## Uninstall default installation
	@echo "🗑️  Uninstalling Gthulhu..."
	helm uninstall $(RELEASE_NAME) --namespace $(NAMESPACE) || true

uninstall-dev: ## Uninstall development installation
	@echo "🗑️  Uninstalling Gthulhu development..."
	helm uninstall $(RELEASE_NAME)-dev --namespace $(NAMESPACE)-dev || true

uninstall-test: ## Uninstall testing installation
	@echo "🗑️  Uninstalling Gthulhu testing..."
	helm uninstall $(RELEASE_NAME)-test --namespace $(NAMESPACE)-test || true

uninstall-all: uninstall uninstall-dev uninstall-test ## Uninstall all installations

clean: uninstall-all ## Clean up everything
	@echo "🧹 Cleaning up namespaces..."
	kubectl delete namespace $(NAMESPACE)-dev --ignore-not-found=true
	kubectl delete namespace $(NAMESPACE)-test --ignore-not-found=true

logs: ## Show logs from all components
	@echo "📜 Showing Gthulhu logs..."
	@echo "=== Scheduler Logs ==="
	kubectl logs -l app.kubernetes.io/component=scheduler --namespace $(NAMESPACE) --tail=50 || true
	@echo ""
	@echo "=== API Logs ==="
	kubectl logs -l app.kubernetes.io/component=api --namespace $(NAMESPACE) --tail=50 || true

logs-dev: ## Show logs from development environment
	@echo "📜 Showing Gthulhu development logs..."
	kubectl logs -l app.kubernetes.io/name=gthulhu --namespace $(NAMESPACE)-dev --tail=50 || true

port-forward: ## Port forward to API service
	@echo "🔗 Port forwarding to API service (http://localhost:8080)..."
	kubectl port-forward svc/$(RELEASE_NAME)-api 8080:80 --namespace $(NAMESPACE)

port-forward-dev: ## Port forward to development API service
	@echo "🔗 Port forwarding to development API service..."
	kubectl get svc -n $(NAMESPACE)-dev | grep gthulhu || echo "No service found"
	kubectl port-forward svc/$(RELEASE_NAME)-dev-api 8080:80 --namespace $(NAMESPACE)-dev

health-check: ## Check health of all components
	@echo "💚 Checking Gthulhu health..."
	@echo "=== Pods ==="
	kubectl get pods -l app.kubernetes.io/name=gthulhu --namespace $(NAMESPACE) || true
	@echo ""
	@echo "=== Services ==="
	kubectl get svc -l app.kubernetes.io/name=gthulhu --namespace $(NAMESPACE) || true
	@echo ""
	@echo "=== DaemonSets ==="
	kubectl get daemonset -l app.kubernetes.io/name=gthulhu --namespace $(NAMESPACE) || true
	@echo ""
	@echo "=== Deployments ==="
	kubectl get deployment -l app.kubernetes.io/name=gthulhu --namespace $(NAMESPACE) || true

package: ## Package the Helm chart
	@echo "📦 Packaging Helm chart..."
	helm package $(CHART_NAME)

dependencies: ## Update chart dependencies
	@echo "📥 Updating chart dependencies..."
	helm dependency update $(CHART_NAME)

docs: ## Generate documentation
	@echo "📚 Generating documentation..."
	helm-docs $(CHART_NAME)

# Development helpers
dev-setup: install-dev ## Set up development environment
	@echo "🛠️  Development environment is ready!"
	@echo "Use 'make port-forward-dev' to access the API"

test-api: ## Test API endpoints (requires port-forward)
	@echo "🧪 Testing API endpoints..."
	@echo "Testing health endpoint..."
	curl -f http://localhost:8080/health || echo "Health check failed"
	@echo ""
	@echo "Testing info endpoint..."
	curl -f http://localhost:8080/ || echo "Info endpoint failed"
	@echo ""
	@echo "Testing metrics endpoint..."
	curl -X POST http://localhost:8080/api/v1/metrics \
		-H "Content-Type: application/json" \
		-d '{"usersched_pid":1234,"nr_queued":10,"nr_scheduled":5,"nr_running":2,"nr_online_cpus":8,"nr_user_dispatches":100,"nr_kernel_dispatches":50,"nr_cancel_dispatches":2,"nr_bounce_dispatches":1,"nr_failed_dispatches":0,"nr_sched_congested":3}' \
		|| echo "Metrics endpoint failed"

# CI/CD helpers
ci-test: lint test ## Run CI tests
	@echo "✅ CI tests completed successfully"

release-check: lint package ## Check if chart is ready for release
	@echo "🚀 Chart is ready for release"
