#!/bin/bash
set -e

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

# Remove generated kubeconfigs (keep README.md)
# Assuming script is run from infra/scripts/
rm -f ../../kubeconfig/*.yaml
rm -f ../../kubeconfig/*.kubeconfig

# Remove CSR directory (relative to root)
rm -rf ../../csr/

# Remove any root-level leftovers (legacy)
rm -f ../../admin.conf
rm -f ../../nginx-deployer.kubeconfig


echo "Done."
