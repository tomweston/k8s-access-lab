#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

CP_VM="cp-1"
WORKER_VMS=("worker-1" "worker-2")
K8S_VERSION="v1.32"

log_step() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

log_info() {
    echo -e "${GREEN}➜ $1${NC}"
}

log_warn() {
    echo -e "${YELLOW}➜ $1${NC}"
}

# 1. Provision VMs
log_step "Provisioning Virtual Machines"
log_info "Launching Control Plane ($CP_VM)..."
multipass launch 22.04 --name $CP_VM --cpus 2 --memory 2G --disk 10G 2>/dev/null || log_warn "$CP_VM already exists"

for vm in "${WORKER_VMS[@]}"; do
    log_info "Launching Worker ($vm)..."
    multipass launch 22.04 --name $vm --cpus 1 --memory 2G --disk 10G 2>/dev/null || log_warn "$vm already exists"
done

# 2. Configure VMs
log_step "Configuring Nodes (Containerd + Kubeadm)"
for vm in "$CP_VM" "${WORKER_VMS[@]}"; do
    log_info "Setting up dependencies on $vm (this may take a few minutes)..."
    multipass exec $vm -- env K8S_VERSION="$K8S_VERSION" bash -s <<'EOF'
        # Wait for cloud-init to finish to avoid apt locks
        echo "Waiting for cloud-init to finish..."
        cloud-init status --wait >/dev/null 2>&1
        
        export DEBIAN_FRONTEND=noninteractive
        export K8S_VERSION="$K8S_VERSION"
        
        # Install containerd
        echo "Installing containerd..."
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor --yes --batch -o /usr/share/keyrings/docker-archive-keyring.gpg
        echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
        sudo apt-get update >/dev/null && sudo apt-get install -y containerd.io >/dev/null
        containerd config default | sudo tee /etc/containerd/config.toml >/dev/null
        sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
        sudo systemctl restart containerd

        # Install kubeadm
        echo "Installing kubeadm/kubectl..."
        curl -fsSL https://pkgs.k8s.io/core:/stable:/$K8S_VERSION/deb/Release.key | sudo gpg --dearmor --yes --batch -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
        echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/$K8S_VERSION/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list >/dev/null
        sudo apt-get update >/dev/null && sudo apt-get install -y kubelet kubeadm kubectl >/dev/null

        # Sysctl & modules
        echo "Configuring sysctl..."
        sudo modprobe overlay
        sudo modprobe br_netfilter
        echo -e "overlay\nbr_netfilter" | sudo tee /etc/modules-load.d/k8s.conf >/dev/null
        echo -e "net.bridge.bridge-nf-call-iptables=1\nnet.bridge.bridge-nf-call-ip6tables=1\nnet.ipv4.ip_forward=1" | sudo tee /etc/sysctl.d/k8s.conf >/dev/null
        sudo sysctl --system >/dev/null
        sudo swapoff -a
EOF
done

# 3. Initialize Cluster
log_step "Initializing Control Plane"
CP_IP=$(multipass info $CP_VM | grep IPv4 | awk '{print $2}')
log_info "Control Plane IP: $CP_IP"

log_info "Running kubeadm init..."
multipass exec $CP_VM -- sudo kubeadm init --pod-network-cidr=10.244.0.0/16 --apiserver-advertise-address=$CP_IP --ignore-preflight-errors=Swap

# 4. Get Kubeconfig
log_step "Retrieving Kubeconfig"
mkdir -p ../../kubeconfig
multipass exec $CP_VM -- sudo cat /etc/kubernetes/admin.conf > ../../kubeconfig/admin.yaml
export KUBECONFIG=$(pwd)/../../kubeconfig/admin.yaml
chmod 600 ../../kubeconfig/admin.yaml
log_info "Saved to kubeconfig/admin.yaml"

# 5. Join Workers
log_step "Joining Workers to Cluster"
JOIN_CMD=$(multipass exec $CP_VM -- sudo kubeadm token create --print-join-command)

for vm in "${WORKER_VMS[@]}"; do
    log_info "Joining $vm..."
    multipass exec $vm -- sudo $JOIN_CMD
done

# 6. Verify
log_step "Cluster Status"
kubectl get nodes -o wide
