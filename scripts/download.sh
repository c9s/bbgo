#!/bin/bash
osf=$(uname | tr '[:upper:]' '[:lower:]')
version=v1.5.0

echo "Downloading bbgo"
curl -L -o bbgo https://github.com/c9s/bbgo/releases/download/$version/bbgo-$osf
chmod +x bbgo
echo "Binary downloaded"
