# Kubernetes Access Lab

Secure Nginx deployment on a kubeadm cluster using Pulumi and a restricted user.

Docs: [Design Document & Tradeoffs](docs/DESIGN.md)

Prereqs: `multipass`, `kubectl`, `ngrok`, `pulumi`, Go 1.21+

## Quick Start

```bash
# 1) Infra
cd infra/scripts
./setup.sh
cd ../..

# 2) Admin stack
cd infra/admin
pulumi org set-default <name-of-org>
pulumi stack init admin
pulumi config set kubeconfig ../../kubeconfig/admin.yaml
pulumi up
pulumi stack output nginxDeployerKubeconfig --show-secrets > ../../kubeconfig/nginx-deployer.yaml
cd ../..

# 3) App stack (ngrok only)
cd apps/nginx
pulumi org set-default <name-of-org>
pulumi stack init app
pulumi config set kubeconfig ../../kubeconfig/nginx-deployer.yaml
pulumi config set host abc123.ngrok-free.app
pulumi config set sslRedirect false
pulumi up
cd ../..

# 4) Verify via ngrok
IP=$(multipass info cp-1 | grep IPv4 | awk '{print $2}')
INGRESS_SVC=$(kubectl get svc -n ingress-nginx \
  -l app.kubernetes.io/component=controller,app.kubernetes.io/name=ingress-nginx \
  -o jsonpath='{.items[0].metadata.name}')
NODEPORT=$(kubectl get svc -n ingress-nginx "$INGRESS_SVC" -o jsonpath='{.spec.ports[?(@.port==80)].nodePort}')
HOST=abc123.ngrok-free.app
ngrok http https://$IP:$NODEPORT --host-header="$HOST"
```

## Teardown

```bash
cd infra/scripts
./teardown.sh
cd ../..
```

## Admin Cleanup (only if re-running)

```bash
export KUBECONFIG=/Users/tom/src/github.com/tomweston/k8s-access-lab/kubeconfig/admin.yaml
kubectl delete validatingwebhookconfiguration -l app.kubernetes.io/name=cert-manager 2>/dev/null || true
kubectl delete mutatingwebhookconfiguration -l app.kubernetes.io/name=cert-manager 2>/dev/null || true
kubectl delete crd certificaterequests.cert-manager.io certificates.cert-manager.io \
  challenges.acme.cert-manager.io clusterissuers.cert-manager.io \
  issuers.cert-manager.io orders.acme.cert-manager.io
```

