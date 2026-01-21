#!/bin/bash
set -e

CP_VM="cp-1"
WORKER_VMS=("worker-1" "worker-2")
K8S_VERSION="v1.32"

# Provision VMs
for vm in "$CP_VM" "${WORKER_VMS[@]}"; do
    multipass launch 22.04 --name $vm --cpus 2 --memory 2G --disk 5G || true
done

# Configure all VMs
for vm in "$CP_VM" "${WORKER_VMS[@]}"; do
    multipass exec $vm -- env K8S_VERSION="$K8S_VERSION" bash -s <<'EOF'
        export K8S_VERSION="$K8S_VERSION"
        # Install containerd
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor --yes --batch -o /usr/share/keyrings/docker-archive-keyring.gpg
        echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list
        sudo apt-get update && sudo apt-get install -y containerd.io
        containerd config default | sudo tee /etc/containerd/config.toml
        sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
        sudo systemctl restart containerd

        # Install kubeadm
        curl -fsSL https://pkgs.k8s.io/core:/stable:/$K8S_VERSION/deb/Release.key | sudo gpg --dearmor --yes --batch -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
        echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/$K8S_VERSION/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list
        sudo apt-get update && sudo apt-get install -y kubelet kubeadm kubectl

        # Sysctl & modules
        sudo modprobe overlay
        sudo modprobe br_netfilter
        echo -e "overlay\nbr_netfilter" | sudo tee /etc/modules-load.d/k8s.conf
        echo -e "net.bridge.bridge-nf-call-iptables=1\nnet.bridge.bridge-nf-call-ip6tables=1\nnet.ipv4.ip_forward=1" | sudo tee /etc/sysctl.d/k8s.conf
        sudo sysctl --system
        sudo swapoff -a
EOF
done

# Initialize cluster
CP_IP=$(multipass info $CP_VM | grep IPv4 | awk '{print $2}')
multipass exec $CP_VM -- sudo kubeadm init --pod-network-cidr=10.244.0.0/16 --apiserver-advertise-address=$CP_IP --ignore-preflight-errors=Swap

# Get kubeconfig
mkdir -p ../../kubeconfig
multipass exec $CP_VM -- sudo cat /etc/kubernetes/admin.conf > ../../kubeconfig/admin.yaml
export KUBECONFIG=$(pwd)/../../kubeconfig/admin.yaml
chmod 600 ../../kubeconfig/admin.yaml

# Join workers
JOIN_CMD=$(multipass exec $CP_VM -- sudo kubeadm token create --print-join-command)
for vm in "${WORKER_VMS[@]}"; do
    multipass exec $vm -- sudo $JOIN_CMD
done

kubectl get nodes
