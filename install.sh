#!/bin/bash
set -e

INSTALL_DIR="/opt/proxy-go2.0"
SRC_DIR="$INSTALL_DIR/src"
BIN="$INSTALL_DIR/proxy"
LOG_DIR="/var/log"

echo "=== PROXY-GO2.0 (PERSISTENTE + LOGS) ==="

sudo rm -rf "$INSTALL_DIR"
sudo mkdir -p "$INSTALL_DIR" "$SRC_DIR" "$LOG_DIR"
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
sudo chown root:root "$BIN"
sudo touch /opt/proxy-go2.0/ports.json
sudo chown root:root /opt/proxy-go2.0/ports.json

sudo tee /etc/systemd/system/proxy-go2.0.service > /dev/null << EOF
[Unit]
Description=Proxy Go 2.0
After=network.target

[Service]
ExecStart=$BIN
WorkingDirectory=$INSTALL_DIR
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now proxy-go2.0

echo "INSTALADO COM SUCESSO!"
echo "Menu: sudo $BIN"
echo "Logs: sudo tail -f /var/log/proxy-go2.0-*.log"
