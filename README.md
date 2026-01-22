# Kubernetes Access Lab

Secure Nginx deployment on a `kubeadm` cluster using Pulumi and a restricted user.

[Design Document & Tradeoffs](docs/DESIGN.md) | [Prerequisites](#prerequisites) | [Quick Start](#quick-start) | [Teardown](#teardown)

---

## Prerequisites

Ensure you have the following installed:
- [**multipass**](https://multipass.run/install) - VM management
- [**kubectl**](https://kubernetes.io/docs/tasks/tools/) - Kubernetes CLI
- [**pulumi**](https://www.pulumi.com/docs/get-started/install/) - Infrastructure as Code
- [**Go 1.21+**](https://go.dev/doc/install) - Backend language

**System Requirements:**
- **RAM:** 6GB (2GB per VM)
- **CPU:** 4 Cores (CP: 2, Workers: 1)
- **Disk:** ~30GB (10GB per VM)

**Install Prerequisites:**
```bash
make deps
```

---

## Quick Start

> **Troubleshooting:** If the automated setup fails, refer to the [Manual Setup Guide](docs/MANUAL_SETUP.md) for step-by-step instructions.

### 1. Provision Cluster
Initialize the virtual machines and bootstrap the `kubeadm` cluster.

```bash
make infra

# Verify VMs are running
multipass list

# Verify nodes are ready
export KUBECONFIG=$(pwd)/kubeconfig/admin.yaml
kubectl get nodes
```

### 2. Bootstrap Admin Layer
Deploy core platform components and generate the restricted deployer credentials.

```bash
cd infra/admin
pulumi stack init admin
pulumi config set kubeconfig ../../kubeconfig/admin.yaml
pulumi up --yes

# Export restricted kubeconfig
pulumi stack output nginxDeployerKubeconfig --show-secrets > ../../kubeconfig/nginx-deployer.yaml
cd ../..
```

### 3. Deploy Application
Using the **restricted** credentials, deploy the Nginx application.

```bash
cd apps/nginx
pulumi stack init app
pulumi config set kubeconfig ../../kubeconfig/nginx-deployer.yaml
pulumi config set host nginx.local
pulumi config set sslRedirect false
pulumi up --yes
cd ../..
```

### 4. Access Application
Access the deployed application directly via the NodePort.

1. **Get the Node IP and Ingress Port:**
   ```bash
   export NODE_IP=$(multipass info cp-1 | grep IPv4 | awk '{print $2}')
   export NODE_PORT=$(kubectl --kubeconfig=kubeconfig/admin.yaml get svc -n ingress-nginx -l app.kubernetes.io/component=controller -o jsonpath='{.items[0].spec.ports[?(@.port==80)].nodePort}')
   echo "Ingress is listening on $NODE_IP:$NODE_PORT"
   ```

2. **Option A: Quick Test (curl)**
   Verify connectivity from your terminal:
   ```bash
   curl -v -H "Host: nginx.local" http://$NODE_IP:$NODE_PORT
   ```

3. **Option B: Browser Access**
   Add the following line to your `/etc/hosts` file (requires sudo):
   ```
   <NODE_IP> nginx.local
   ```
   Replace `<NODE_IP>` with the actual IP from step 1. Then open `http://nginx.local:<NODE_PORT>` in your browser.

---

## Teardown

Clean up resources in reverse order to ensure clean deletion.

```bash
make clean
```

## Author

[![LinkedIn](https://img.shields.io/badge/linkedin-%230077B5.svg?&style=for-the-badge&logo=linkedin&logoColor=white)](https://www.linkedin.com/in/westontom)
[![Twitter](https://img.shields.io/badge/@tomweston-%231DA1F2.svg?&style=for-the-badge&logo=x&logoColor=white)](https://twitter.com/tomweston)