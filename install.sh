#!/bin/bash

# install.sh - Instalador Proxy Go 2.0
# Verifica Ubuntu 18/20/22/24, instala Go, OpenSSH, compila TODO o projeto e configura systemd

set -e

# Cores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[AVISO]${NC} $1"; }
error() { echo -e "${RED}[ERRO]${NC} $1" >&2; }

echo -e "${GREEN}=== Instalador Proxy Go 2.0 ===${NC}"

# === 1. Verifica Ubuntu suportado ===
if [ ! -f /etc/os-release ]; then
    error "/etc/os-release não encontrado."
    exit 1
fi

source /etc/os-release
if [[ "$ID" != "ubuntu" ]]; then
    error "Suporte apenas para Ubuntu. Detectado: $ID"
    exit 1
fi

case "$VERSION_ID" in
    "18.04"|"20.04"|"22.04"|"24.04") log "Ubuntu $VERSION_ID detectado. OK." ;;
    *) error "Versão Ubuntu não suportada: $VERSION_ID"; exit 1 ;;
esac

# === 2. Define diretório de instalação ===
INSTALL_DIR="/opt/proxy-go2.0"
SRC_DIR="$INSTALL_DIR/src"
BIN_DIR="$INSTALL_DIR"
BIN_PATH="$BIN_DIR/proxy"

log "Diretório de instalação: $INSTALL_DIR"

mkdir -p "$INSTALL_DIR" "$SRC_DIR"
cd "$INSTALL_DIR"

# === 3. Atualiza sistema e instala dependências ===
log "Atualizando pacotes..."
apt update -qq && apt upgrade -y

# Instala git, curl, wget, ca-certificates
for pkg in git curl wget ca-certificates; do
    if ! dpkg -l | grep -q "^ii  $pkg "; then
        log "Instalando $pkg..."
        apt install -y "$pkg"
    fi
done

# === 4. Instala Go (se necessário) ===
if ! command -v go &> /dev/null; then
    log "Instalando Go 1.22..."
    wget -q https://go.dev/dl/go1.22.8.linux-amd64.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf go1.22.8.linux-amd64.tar.gz
    rm go1.22.8.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    export PATH=$PATH:/usr/local/go/bin
fi

log "Go versão: $(go version)"

# === 5. Instala OpenSSH (para autenticação) ===
if ! systemctl is-active --quiet ssh; then
    log "Instalando OpenSSH Server..."
    apt install -y openssh-server
    systemctl enable ssh
    systemctl start ssh
fi

# === 6. Clona o repositório (ou atualiza) ===
if [ -d "$SRC_DIR/.git" ]; then
    log "Atualizando repositório existente..."
    cd "$SRC_DIR"
    git pull origin main
else
    log "Clonando repositório..."
    rm -rf "$SRC_DIR"
    git clone https://github.com/jeanfraga95/proxy-go2.0.git "$SRC_DIR"
    cd "$SRC_DIR"
fi

# === 7. Compila TODO o projeto (resolve erros de import e undefined) ===
log "Compilando o proxy (todos os arquivos .go)..."

cd "$SRC_DIR"

# Garante dependências
go mod tidy

# Compila tudo com nome fixo
go build -o "$BIN_PATH" .

if [ ! -f "$BIN_PATH" ]; then
    error "Falha ao compilar o binário. Verifique os arquivos .go"
    exit 1
fi

chmod +x "$BIN_PATH"
log "Binário compilado: $BIN_PATH"

# === 8. Configura serviço systemd ===
cat > /etc/systemd/system/proxy-go2.0.service << EOF
[Unit]
Description=Proxy Go 2.0 - WSS + SOCKS5 com autenticação SSH
After=network.target ssh.service

[Service]
Type=simple
User=root
WorkingDirectory=$BIN_DIR
ExecStart=$BIN_PATH
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable proxy-go2.0.service

# === 9. Inicia o serviço (opcional no boot) ===
if systemctl is-active --quiet proxy-go2.0; then
    log "Serviço já ativo."
else
    log "Iniciando serviço proxy-go2.0..."
    systemctl start proxy-go2.0
fi

# === 10. Finalização ===
echo -e "${GREEN}"
echo "╔══════════════════════════════════════════════════════════╗"
echo "║             INSTALAÇÃO CONCLUÍDA COM SUCESSO!             ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo -e "${NC}"
echo -e "${YELLOW}Comandos úteis:${NC}"
echo "   • Menu interativo:   sudo $BIN_PATH"
echo "   • Iniciar serviço:   sudo systemctl start proxy-go2.0"
echo "   • Parar serviço:     sudo systemctl stop proxy-go2.0"
echo "   • Ver logs:          sudo journalctl -u proxy-go2.0 -f"
echo "   • Status:            sudo systemctl status proxy-go2.0"
echo ""

log "Proxy Go 2.0 instalado e rodando!"
