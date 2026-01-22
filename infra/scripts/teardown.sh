#!/bin/bash
set -e

# Resolve project root relative to this script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/../.."

echo "=== Teardown ==="
echo "This will delete all VMs (cp-1, worker-1, worker-2) and local configs. Are you sure? (y/N)"
read -r confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "Aborted."
    exit 0
fi

echo "Deleting VMs..."
multipass delete cp-1 worker-1 worker-2 --purge || true

echo "Cleaning up files..."

# Remove generated kubeconfigs
rm -f "$PROJECT_ROOT/kubeconfig/"*.yaml
rm -f "$PROJECT_ROOT/kubeconfig/"*.kubeconfig

# Remove CSR directory
rm -rf "$PROJECT_ROOT/csr/"

# Remove any root-level leftovers (legacy)
rm -f "$PROJECT_ROOT/admin.conf"
rm -f "$PROJECT_ROOT/nginx-deployer.kubeconfig"

echo "Done."
