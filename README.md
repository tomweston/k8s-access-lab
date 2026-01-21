# Kubernetes Access Lab

Secure Nginx deployment on a `kubeadm` cluster using Pulumi and a restricted user.

[Design Document & Tradeoffs](docs/DESIGN.md) | [Prerequisites](#prerequisites) | [Quick Start](#quick-start) | [Teardown](#teardown)

---

## Prerequisites

Ensure you have the following installed:
- [**multipass**](https://multipass.run/install) - VM management
- [**kubectl**](https://kubernetes.io/docs/tasks/tools/) - Kubernetes CLI
- [**pulumi**](https://www.pulumi.com/docs/get-started/install/) - Infrastructure as Code
- [**ngrok**](https://ngrok.com/download) - Public ingress tunnel
- [**Go 1.21+**](https://go.dev/doc/install) - Backend language

**System Requirements:**
- **RAM:** 5GB (CP: 2GB, Workers: 1.5GB)
- **CPU:** 4 Cores (CP: 2, Workers: 1)
- **Disk:** ~15GB (3 VMs x 5GB)

**Install Prerequisites:**
```bash
make deps
```

---

## Quick Start

### 1. Provision Cluster
Initialize the virtual machines and bootstrap the `kubeadm` cluster.

```bash
make infra

# Verify nodes are ready
export KUBECONFIG=$(pwd)/kubeconfig/admin.yaml
kubectl get nodes
```

### 2. Bootstrap Admin Layer
Deploy core platform components and generate the restricted deployer credentials.

```bash
make admin
```

### 3. Deploy Application
Using the **restricted** credentials, deploy the Nginx application.

```bash
make app
```

### 4. Access via ngrok
Expose the ingress controller to the internet to verify the deployment.

```bash
make tunnel
```

---

## Teardown

Clean up resources in reverse order to ensure clean deletion.

```bash
make clean
```

## Author

[![LinkedIn](https://img.shields.io/badge/linkedin-%230077B5.svg?&style=for-the-badge&logo=linkedin&logoColor=white)](https://www.linkedin.com/in/westontom)
[![Twitter](https://img.shields.io/badge/@tomweston-%231DA1F2.svg?&style=for-the-badge&logo=x&logoColor=white)](https://twitter.com/tomweston)