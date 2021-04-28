#!/bin/bash
set -e
osf=$(uname | tr '[:upper:]' '[:lower:]')
version=v1.15.2
dist_file=bbgo-$version-$osf-amd64.tar.gz

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

function warn()
{
    echo -e "${YELLOW}$@${NC}"
}

function error()
{
    echo -e "${RED}$@${NC}"
}

function info()
{
    echo -e "${GREEN}$@${NC}"
}

info "downloading..."
curl -O -L https://github.com/c9s/bbgo/releases/download/$version/$dist_file
tar xzf $dist_file
mv bbgo-$osf bbgo
chmod +x bbgo
info "downloaded successfully"
