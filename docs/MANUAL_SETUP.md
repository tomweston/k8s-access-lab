# Manual Cluster Setup Guide

If the automated `make infra` process fails, you can follow these steps to bootstrap the cluster manually. This guide explains the role of each component.

## 1. Provision VMs
Create 3 Ubuntu 22.04 VMs using Multipass.

```bash
multipass launch 22.04 --name cp-1 --cpus 2 --memory 2G --disk 10G
multipass launch 22.04 --name worker-1 --cpus 1 --memory 2G --disk 10G
multipass launch 22.04 --name worker-2 --cpus 1 --memory 2G --disk 10G
```

## 2. Node Preparation (Run on ALL Nodes)
Perform these steps for **each** node (`cp-1`, `worker-1`, `worker-2`).
Set the `VM` variable and run the blocks below:

```bash
# Example: Run for cp-1, then repeat for worker-1 and worker-2
VM=cp-1
```

### A. Install Container Runtime (containerd)
Kubernetes needs a runtime to launch containers. We use `containerd`.

```bash
multipass exec $VM -- sudo bash -c '
  # Install dependencies
  apt-get update && apt-get install -y containerd.io

  # Generate default config
  containerd config default > /etc/containerd/config.toml

  # Enable Systemd Cgroups
  sed -i "s/SystemdCgroup = false/SystemdCgroup = true/" /etc/containerd/config.toml
  systemctl restart containerd
'
```

### B. Install Kubernetes Tools
- **kubeadm**: The bootstrapper. Handles certs, etcd, and component setup.
- **kubelet**: The node agent. Starts pods and reports status to the API server.
- **kubectl**: The CLI for talking to the cluster.

```bash
multipass exec $VM -- sudo bash -c '
  # Set Kubernetes version
  K8S_VERSION=v1.32

  # Add K8s repo
  curl -fsSL https://pkgs.k8s.io/core:/stable:/$K8S_VERSION/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/$K8S_VERSION/deb/ /" > /etc/apt/sources.list.d/kubernetes.list

  apt-get update
  apt-get install -y kubelet kubeadm kubectl
'
```

### C. System Configuration
Networking requirements and swap handling.

```bash
multipass exec $VM -- sudo bash -c '
  # Load kernel modules for container networking
  modprobe overlay
  modprobe br_netfilter

  # Enable IP forwarding (allows pods to talk to each other across nodes)
  echo -e "net.bridge.bridge-nf-call-iptables=1\nnet.ipv4.ip_forward=1" > /etc/sysctl.d/k8s.conf
  sysctl --system

  # Disable Swap (K8s scheduler requires this for performance predictability)
  swapoff -a
'
```

## 3. Bootstrap Control Plane (Run on `cp-1`)
Initialize the master node.

```bash
multipass exec cp-1 -- sudo kubeadm init --pod-network-cidr=10.244.0.0/16
```

**What `kubeadm init` does:**
1.  **Preflight Checks**: Validates system state (RAM, CPU, ports).
2.  **Certs**: Generates a self-signed CA and certificates for API server, etcd, etc.
3.  **Kubeconfig**: Generates admin credentials (`/etc/kubernetes/admin.conf`).
4.  **Control Plane**: Starts static pods (API Server, Controller Manager, Scheduler, Etcd).
5.  **Bootstrap Token**: Generates the token for workers to join.

## 4. Join Workers (Run on `worker-1`, `worker-2`)
Connect the worker nodes to the cluster.

```bash
# 1. Get the join command from the control plane
JOIN_CMD=$(multipass exec cp-1 -- sudo kubeadm token create --print-join-command)

# 2. Run it on the workers
multipass exec worker-1 -- sudo $JOIN_CMD
multipass exec worker-2 -- sudo $JOIN_CMD
```

**What `kubeadm join` does:**
1.  **Discovery**: Connects to API server to fetch cluster info (verified by CA hash).
2.  **TLS Bootstrap**: Generates a private key and requests a certificate from the API server.
3.  **Registration**: The node registers itself and the `kubelet` starts waiting for work.

## 5. Post-Setup
On your host machine, retrieve the admin kubeconfig to interact with the cluster:

```bash
mkdir -p kubeconfig
multipass exec cp-1 -- sudo cat /etc/kubernetes/admin.conf > kubeconfig/admin.yaml
```
