# Kubernetes Access Lab

A reference implementation for deploying a secure, RBAC-restricted Nginx application on a Kubeadm-bootstrapped Kubernetes cluster using Pulumi.

## Overview

This project provides a "one-click" experience to:
1.  **Provision** 3 VMs (Multipass)
2.  **Bootstrap** a Kubernetes cluster (kubeadm)
3.  **Deploy** an Nginx application as a restricted user (RBAC + CSR) using **Pulumi**.

## Architecture & Tool Choice

This project prioritizes a realistic, "bare-metal" simulation over the convenience of pre-packaged Kubernetes distributions.

## Prerequisites

*   **Multipass**: `brew install multipass`
*   **kubectl**: `brew install kubectl`
*   **ngrok**: `brew install ngrok` 
*   **Pulumi + Go**: `brew install pulumi`, 

## Quick Start

<details>
<summary>1. Setup Infrastructure</summary>

This script handles the physical infrastructure: provisioning VMs, installing Containerd/Kubeadm, initializing the cluster, and joining workers.

**Note:** The cluster nodes will remain in `NotReady` state until the CNI (Flannel) is installed by the Pulumi Admin stack in the next step.

```bash
cd infra/scripts
./setup.sh
cd ../..
```

**Output:**
- Creates `kubeconfig/admin.yaml` (Admin Kubeconfig) in the repo root
- Displays Cluster Nodes (Note: Nodes will be `NotReady` until Pulumi Admin stack runs)

</details>

<details>
<summary>2. Admin Stack</summary>

This stack creates the namespace, RBAC, and the cert-manager ClusterIssuer using admin credentials:

```bash
cd infra/admin
pulumi org set-default <name-of-org>
pulumi stack init admin
pulumi config set kubeconfig ../../kubeconfig/admin.yaml
pulumi up
cd ../..
```

**Creates:**
- **Networking**: Flannel CNI (makes nodes Ready)
- **Platform**: Ingress-Nginx Controller, Cert-Manager
- **Config**: Namespace `app-nginx`, RBAC role/binding for `nginx-deployer`, ClusterIssuer

</details>

<details>
<summary>3. Use ngrok Hostname</summary>

If you do not want to edit your local hosts file, you can use an ngrok
hostname and set the app stack `host` to that value. This gives you a
public URL that matches the Ingress host.

Use the ngrok hostname (without https://) as the `host` in the app stack
configuration in Step 5, and follow Verify Option B to start the ngrok tunnel.

</details>

<details>
<summary>4. Bootstrap Restricted User</summary>

The admin stack installs core platform components (Flannel, ingress-nginx,
cert-manager, RBAC, and the ClusterIssuer). It also generates and approves the
`nginx-deployer` certificate and exports a kubeconfig for the restricted user.
Capture it after `pulumi up`:

```bash
# Run from the repo root
cd infra/admin
pulumi stack output nginxDeployerKubeconfig --show-secrets > ../../kubeconfig/nginx-deployer.yaml
cd ../..
```

</details>

<details>
<summary>5. App Stack</summary>

This stack deploys the application using the **restricted user kubeconfig** generated in the previous step:

```bash
# Run from the repo root
cd apps/nginx
pulumi org set-default <name-of-org>
pulumi stack init app
pulumi config set kubeconfig ../../kubeconfig/nginx-deployer.yaml
# If using ngrok, replace with your ngrok hostname (no https:// prefix)
# Example: pulumi config set host abc123.ngrok-free.app
pulumi config set host abc123.ngrok-free.app
pulumi config set sslRedirect false
pulumi up
cd ../..
```

**Creates:**
- ConfigMap `nginx-html`
- Deployment `nginx` (2 replicas by default)
- Service `nginx`
- Ingress `nginx` (TLS via cert-manager)

</details>

<details>
<summary>6. Verify</summary>

Expose your cluster to the internet using ngrok:

```bash
# Run from the repo root
cd ../..

# Get Control Plane IP
IP=$(multipass info cp-1 | grep IPv4 | awk '{print $2}')

# Get NodePort (dynamic service name)
INGRESS_SVC=$(kubectl get svc -n ingress-nginx \
  -l app.kubernetes.io/component=controller,app.kubernetes.io/name=ingress-nginx \
  -o jsonpath='{.items[0].metadata.name}')
NODEPORT=$(kubectl get svc -n ingress-nginx "$INGRESS_SVC" -o jsonpath='{.spec.ports[?(@.port==80)].nodePort}')

# Start ngrok (match your deployment host)
HOST=abc123.ngrok-free.app
ngrok http https://$IP:$NODEPORT --host-header="$HOST"
```

</details>

<details>
<summary>7. Teardown</summary>

To delete all VMs and config files:

```bash
cd infra/scripts
./teardown.sh
cd ../..
```

</details>

<details>
<summary>Admin Stack Cleanup</summary>

When destroying the admin stack, Helm intentionally preserves cert-manager
CRDs. In some cases, the old webhook configs can also linger. If you want a
fully clean re-run (and to avoid webhook/ownership conflicts), delete the
CRDs and any old webhook configurations after `pulumi destroy`:

```bash
export KUBECONFIG=/Users/tom/src/github.com/tomweston/k8s-access-lab/kubeconfig/admin.yaml

# Remove stale cert-manager webhook configs (if present)
kubectl delete validatingwebhookconfiguration -l app.kubernetes.io/name=cert-manager 2>/dev/null || true
kubectl delete mutatingwebhookconfiguration -l app.kubernetes.io/name=cert-manager 2>/dev/null || true

# Remove cert-manager CRDs kept by Helm
kubectl delete crd certificaterequests.cert-manager.io certificates.cert-manager.io \
  challenges.acme.cert-manager.io clusterissuers.cert-manager.io \
  issuers.cert-manager.io orders.acme.cert-manager.io
```

</details>

## Project Structure

*   `infra/scripts/`: Helper scripts for cluster lifecycle.
    *   `setup.sh`: Infrastructure-as-Code (Bash). Provisions VMs, bootstraps kubeadm cluster, installs add-ons.
    *   `teardown.sh`: Cleanup. Removes all VMs and generated config files.
*   `infra/admin`: Admin Pulumi stack (namespace, RBAC, ClusterIssuer).
*   `apps/nginx`: App Pulumi stack (ConfigMap, Deployment, Service, Ingress).
*   `kubeconfig/`: Directory for generated cluster credentials (ignored by git).
