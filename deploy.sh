#!/bin/bash
set -e

target=$1
bin_dir=bin
bin_type=bbgo-slim
host=bbgo
os=linux
arch=amd64

# use the git describe as the binary version, you may override this with something else.
tag=$(git describe --tags)

RED='\033[1;31m'
GREEN='\033[1;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function warn() {
  echo "${YELLOW}$@${NC}"
}

function error() {
  echo "${RED}$@${NC}"
}

function info() {
  echo "${GREEN}$@${NC}"
}

if [[ -n $BBGO_HOST ]]; then
  host=$BBGO_HOST
else
  warn "env var BBGO_HOST is not set, using \"bbgo\" host alias as the default host, you can add \"bbgo\" to your ~/.ssh/config file"
  host=bbgo
fi

if [[ -z $target ]]; then
  echo "Usage: $0 [target]"
  echo "target name is required"
  exit 1
fi

make $bin_type-$os-$arch

# initialize the remote environment
# create the directory for placing binaries
ssh $host "mkdir -p \$HOME/$bin_dir"

# copy the binary to the server
info "deploying..."
info "copying binary to host $host..."
scp build/bbgo/$bin_type-$os-$arch $host:$bin_dir/bbgo-$tag

# link binary and restart the systemd service
info "linking binary and restarting..."
ssh $host "(cd $target && ln -sf \$HOME/$bin_dir/bbgo-$tag bbgo && systemctl restart $target.service)"

info "deployed successfully!"
