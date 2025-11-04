#!/bin/bash
set -e

INSTALL_DIR="/opt/proxy-go2.0"
SRC_DIR="$INSTALL_DIR/src"
BIN="$INSTALL_DIR/proxy"

echo "=== PROXY-GO2.0 HÍBRIDO (HTTP INJECTOR + WSS) ==="

sudo rm -rf "$INSTALL_DIR"
sudo mkdir -p "$INSTALL_DIR" "$SRC_DIR" "/var/log" "/tmp"
cd "$SRC_DIR"

git clone https://github.com/jeanfraga95/proxy-go2.0.git .
cd ..

if ! command -v go &> /dev/null; then
    wget -q https://go.dev/dl/go1.22.8.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.22.8.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
    source /etc/profile.d/go.sh
fi

cd "$SRC_DIR"
go mod tidy
go build -o "$BIN" .

sudo chmod +x "$BIN"
sudo touch "$INSTALL_DIR/ports.json"
sudo chown root:root "$INSTALL_DIR/ports.json"

echo "INSTALADO!"
echo "HTTP Injector: SOCKS5 → SEU_IP:80"
echo "WSS: wss://SEU_IP:80 (use --no-check)"
