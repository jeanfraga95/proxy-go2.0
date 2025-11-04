#!/bin/bash
set -e

INSTALL_DIR="/opt/proxy-go2.0"
SRC_DIR="$INSTALL_DIR/src"
BIN="$INSTALL_DIR/proxy"

echo "=== Instalando Proxy Go 2.0 ==="

# Verifica Ubuntu
source /etc/os-release
if [[ "$ID" != "ubuntu" ]] || ! [[ "$VERSION_ID" =~ ^(18.04|20.04|22.04|24.04)$ ]]; then
    echo "Erro: apenas Ubuntu 18/20/22/24"
    exit 1
fi

# Remove instalação anterior
sudo rm -rf "$INSTALL_DIR"

# Cria diretórios
mkdir -p "$SRC_DIR"
cd "$SRC_DIR"

# Clona repo
git clone https://github.com/jeanfraga95/proxy-go2.0.git .
cd ..

# Instala Go se necessário
if ! command -v go &> /dev/null; then
    wget -q https://go.dev/dl/go1.22.8.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.22.8.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
    export PATH=$PATH:/usr/local/go/bin
fi

# Compila
cd "$SRC_DIR"
go mod tidy
go build -o "$BIN" .

chmod +x "$BIN"

# Systemd
sudo tee /etc/systemd/system/proxy-go2.0.service > /dev/null << EOF
[Unit]
Description=Proxy Go 2.0
After=network.target

[Service]
ExecStart=$BIN
WorkingDirectory=$INSTALL_DIR
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now proxy-go2.0

echo "INSTALADO COM SUCESSO!"
echo "Menu: sudo $BIN"
echo "Logs: sudo journalctl -u proxy-go2.0 -f"
