# Design Document: Secure Nginx Deployment

## 1. Overview
This document outlines the architectural decisions, security implementation, and trade-offs for the Kubernetes Access Lab. The goal was to provision a bare-metal Kubernetes cluster and deploy an Nginx application using a restricted user account, strictly adhering to the principle of least privilege.

## 2. Architecture Components

### Infrastructure (Layer 1)
* **Hypervisor:** Multipass (Ubuntu VMs).
* **Orchestrator:** Kubeadm (Standard "bare-metal" bootstrapping).
* **Networking:** Flannel CNI (Overlay network).

### Management (Layer 2)
* **Tooling:** Pulumi (Go SDK).
* **Rationale:** Pulumi was chosen over static YAML or Helm because it allows for **imperative logic** within the infrastructure definition. Specifically, the requirement to generate a private key, create a CSR, submit it to the API, and wait for approval could be automated in a single execution flow using Go, which would be difficult with declarative tools like pure Manifests.

### Application (Layer 3)
* **Ingress:** Nginx Ingress Controller (NodePort service exposed to host).
* **Certificates:** Cert-Manager (Self-Signed ClusterIssuer for TLS).
* **Workload:** Nginx Deployment (ReplicaSet + Service).

---

## 3. Security & User Access Design

### Authentication Strategy: X.509 Client Certificates
Instead of using a static service account token or the admin `kubeconfig`, this solution implements authentic **User** authentication via the Kubernetes Certificate Signing Request (CSR) API.

**The Workflow:**
1.  **Key Generation:** The Admin stack generates a 2048-bit RSA key pair locally.
2.  **CSR Submission:** A `CertificateSigningRequest` is submitted to the K8s API with `CN=nginx-deployer` and `O=nginx-deployers`.
3.  **Approval:** The Admin stack programmatically approves the CSR (simulating an admin action).
4.  **Retrieval:** The signed certificate is retrieved and embedded into a restricted `kubeconfig` file.

### Authorization Strategy: RBAC
Access is strictly scoped using Role-Based Access Control:
* **Role:** `nginx-deployer-role` (Namespaced).
* **Scope:** Restricted solely to the `app-nginx` namespace.
* **Permissions:** `create`, `get`, `list`, `update`, `delete` for specific resources (Deployment, Service, Ingress, ConfigMap).
* **Constraint:** The user **cannot** view nodes, access `kube-system`, or modify cluster-level resources.

---

## 4. Trade-offs & Analysis

### Pros (Why this approach works)
* **Zero-Trust Foundation:** No long-lived passwords. Authentication is based on cryptographic proof (private key).
* **Standardization:** Uses native Kubernetes APIs (CSR, RBAC) without requiring external identity providers for this specific lab environment.
* **Automation:** The entire identity lifecycle—from key generation to kubeconfig creation—is codified. This eliminates the "human error" of manually running `openssl` commands.

### Cons & "Issues with this style of management"
* **Revocation Difficulty:** Hard to revoke quickly without CRL/OCSP support.
* **Rotation Complexity:** Rotation requires regenerating and redistributing kubeconfigs.
* **Identity Drift:** Not tied to a central identity source, so access can persist.
* **Alternative:** OIDC with short-lived tokens is preferred in production.

### Deployment Strategy: Push (Pulumi) vs. GitOps
* **Current Approach (Push):** We use Pulumi to push changes.
    * *Pro:* Immediate feedback, easier for bootstrapping and development.
    * *Con:* Risk of configuration drift if someone manually edits the cluster.
* **Alternative (GitOps):** Tools like ArgoCD.
    * *Pro:* The git repo is the single source of truth; automatic drift correction.
    * *Con:* Adds significant complexity/overhead for a 3-node lab cluster.

## 5. Future Improvements
1.  **OIDC Integration:** Replace Client Certs with `dex` or an OIDC provider for SSO integration.
2.  **External DNS:** Integrate with a real DNS provider instead of using `ngrok` or `/etc/hosts`.
3.  **Network Policies:** Further restrict traffic so that only the Ingress Controller can talk to the Nginx pods.
