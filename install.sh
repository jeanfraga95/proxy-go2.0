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

# Instala dependências
apt install -y git wget openssl openssh-server

# Instala Go
if ! command -v go &> /dev/null; then
    wget -q https://go.dev/dl/go1.22.8.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.22.8.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    export PATH=$PATH:/usr/local/go/bin
fi

# Clona e compila
mkdir -p "$SRC_DIR"
cd "$SRC_DIR"
git clone https://github.com/jeanfraga95/proxy-go2.0.git . || git pull
go mod tidy
go build -o "$BIN" .

chmod +x "$BIN"

# Systemd
cat > /etc/systemd/system/proxy-go2.0.service << EOF
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

systemctl daemon-reload
systemctl enable --now proxy-go2.0

echo "INSTALADO! Use: $BIN"
echo "Logs: journalctl -u proxy-go2.0 -f"
