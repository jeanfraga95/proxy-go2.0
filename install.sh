#!/bin/bash

# install.sh - Instalador Proxy Go 2.0
# Verifica Ubuntu 18/20/22/24, instala deps, configura systemd

set -e

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Instalador Proxy Go 2.0 ===${NC}"

# Verifica distribuição
if [ ! -f /etc/os-release ]; then
    echo -e "${RED}Erro: /etc/os-release não encontrado.${NC}"
    exit 1
fi
source /etc/os-release
if [[ "$ID" != "ubuntu" ]]; then
    echo -e "${RED}Erro: Suporte apenas para Ubuntu 18/20/22/24. Detectado: $ID${NC}"
    exit 1
fi
UBUNTU_VERSION=$(echo "$VERSION_ID" | tr -d '"')
case $UBUNTU_VERSION in
    "18.04"|"20.04"|"22.04"|"24.04") echo -e "${GREEN}Ubuntu $UBUNTU_VERSION detectado. OK.${NC}" ;;
    *) echo -e "${RED}Erro: Versão Ubuntu não suportada: $UBUNTU_VERSION${NC}"; exit 1 ;;
esac

# Diretório de instalação
INSTALL_DIR="/opt/proxy-go2.0"
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

# Atualiza sistema
echo -e "${YELLOW}Atualizando pacotes...${NC}"
apt update && apt upgrade -y

# Instala dependências se necessário
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Instalando Go...${NC}"
    wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
    rm go1.22.0.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    export PATH=$PATH:/usr/local/go/bin
fi

if ! command -v git &> /dev/null; then
    apt install -y git
fi

if ! systemctl is-active --quiet ssh; then
    echo -e "${YELLOW}Instalando e iniciando OpenSSH...${NC}"
    apt install -y openssh-server
    systemctl enable ssh
    systemctl start ssh
fi

# Clona ou baixa código fonte (assumindo repo GitHub)
if [ ! -d "$INSTALL_DIR/src" ]; then
    git clone https://github.com/jeanfraga95/proxy-go2.0.git src
    cd src
    go mod tidy
    go build -o proxy main.go
    mv proxy ..
    cd ..
fi

# Configura systemd service
cat > /etc/systemd/system/proxy-go2.0.service << EOF
[Unit]
Description=Proxy Go 2.0
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/proxy
Restart=always
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable proxy-go2.0

# Permissões
chmod +x install.sh proxy
chmod 755 /etc/systemd/system/proxy-go2.0.service

echo -e "${GREEN}Instalação concluída!${NC}"
echo -e "${YELLOW}Para rodar: systemctl start proxy-go2.0${NC}"
echo -e "${YELLOW}Menu: $INSTALL_DIR/proxy${NC}"
echo -e "${YELLOW}Logs: journalctl -u proxy-go2.0 -f${NC}"
