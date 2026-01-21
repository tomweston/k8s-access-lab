.PHONY: all deps infra admin app tunnel clean

# Default target
all: deps infra admin app

# 0. Prerequisites
deps:
	@if command -v brew >/dev/null 2>&1; then \
		echo "Homebrew detected. Installing dependencies..."; \
		brew bundle; \
	else \
		echo "Homebrew not found. Please install the following manually:"; \
		echo "  - multipass: https://multipass.run/install"; \
		echo "  - kubectl:   https://kubernetes.io/docs/tasks/tools/"; \
		echo "  - pulumi:    https://www.pulumi.com/docs/get-started/install/"; \
		echo "  - ngrok:     https://ngrok.com/download"; \
		echo "  - go:        https://go.dev/doc/install"; \
	fi

# 1. Infrastructure (VMs + K8s)
infra:
	@echo "Provisioning Infrastructure..."
	chmod +x infra/scripts/setup.sh
	./infra/scripts/setup.sh

# 2. Admin Stack (Platform + RBAC)
admin:
	@echo "Deploying Admin Stack..."
	cd infra/admin && pulumi stack select admin || pulumi stack init admin
	cd infra/admin && pulumi config set kubeconfig ../../kubeconfig/admin.yaml
	cd infra/admin && pulumi up --yes
	@echo "Exporting restricted kubeconfig..."
	cd infra/admin && pulumi stack output nginxDeployerKubeconfig --show-secrets > ../../kubeconfig/nginx-deployer.yaml

# 3. App Stack (Nginx)
app:
	@echo "Deploying App Stack..."
	cd apps/nginx && pulumi stack select app || pulumi stack init app
	cd apps/nginx && pulumi config set kubeconfig ../../kubeconfig/nginx-deployer.yaml
	cd apps/nginx && pulumi config set host $$(whoami)-k8s-lab.ngrok-free.app
	cd apps/nginx && pulumi config set sslRedirect false
	cd apps/nginx && pulumi up --yes

# 4. Access (Tunnel)
tunnel:
	@echo "Starting ngrok tunnel..."
	@IP=$$(multipass info cp-1 | grep IPv4 | awk '{print $$2}') && \
	PORT=$$(kubectl get svc -n ingress-nginx -l app.kubernetes.io/component=controller -o jsonpath='{.items[0].spec.ports[?(@.port==80)].nodePort}') && \
	HOST=$$(whoami)-k8s-lab.ngrok-free.app && \
	echo "Tunneling to http://$$HOST -> https://$$IP:$$PORT" && \
	ngrok http https://$$IP:$$PORT --host-header="$$HOST"

# Teardown
clean:
	@echo "Destroying App Stack..."
	-cd apps/nginx && pulumi destroy --yes
	@echo "Destroying Admin Stack..."
	-cd infra/admin && pulumi destroy --yes
	@# Note: cert-manager CRDs (customresourcedefinitions) are preserved by Helm to prevent data loss.
	@# This causes a warning during destroy, but they will be fully removed when the VMs are deleted below.
	@echo "Cleaning up Infrastructure..."
	./infra/scripts/teardown.sh
