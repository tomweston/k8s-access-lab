#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_step() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

log_info() {
    echo -e "${GREEN}➜ $1${NC}"
}

log_warn() {
    echo -e "${YELLOW}➜ $1${NC}"
}

# Resolve project root relative to this script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/../.."

log_step "Teardown"
echo -e "${RED}This will delete all VMs (cp-1, worker-1, worker-2) and local configs.${NC}"
echo -n "Are you sure? (y/N) "
read -r confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    log_info "Aborted."
    exit 0
fi

log_step "Deleting Virtual Machines"
log_info "Deleting cp-1, worker-1, worker-2..."
multipass delete cp-1 worker-1 worker-2 --purge >/dev/null 2>&1 || log_warn "Some VMs were not found (already deleted?)"

log_step "Cleaning up Files"
log_info "Removing local kubeconfigs and certificates..."

# Remove generated kubeconfigs
rm -f "$PROJECT_ROOT/kubeconfig/"*.yaml
rm -f "$PROJECT_ROOT/kubeconfig/"*.kubeconfig

# Remove CSR directory
rm -rf "$PROJECT_ROOT/csr/"

# Remove any root-level leftovers (legacy)
rm -f "$PROJECT_ROOT/admin.conf"
rm -f "$PROJECT_ROOT/nginx-deployer.kubeconfig"

log_info "Done."
