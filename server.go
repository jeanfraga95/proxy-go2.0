// server.go
package main

import (
    "crypto/tls"
    "fmt"
    "io"
    "log"
    "net"
    "net/http"
    "os"

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func StartProxy(port int) error {
    // Gera cert TLS auto-assinado se não existir
    certFile := fmt.Sprintf("cert-%d.pem", port)
    keyFile := fmt.Sprintf("key-%d.pem", port)
    if _, err := os.Stat(certFile); os.IsNotExist(err) {
        generateSelfSignedCert(certFile, keyFile)
    }

    // Config TLS
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return err
    }
    tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

    // Listener TCP na porta (compartilhado para HTTP/WS e SOCKS)
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        return err
    }
    defer listener.Close()

    log.Printf("Proxy iniciado na porta %d (WSS + SOCKS5 com auth SSH)\n", port)

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Printf("Erro accept: %v", err)
            continue
        }
        go handleConnection(conn, tlsConfig)
    }
}

func handleConnection(rawConn net.Conn, tlsConfig *tls.Config) {
    defer rawConn.Close()

    // TLS handshake (para WSS ou SOCKS over TLS)
    conn := tls.Server(rawConn, tlsConfig)
    if err := conn.Handshake(); err != nil {
        log.Printf("TLS Handshake falhou: %v", err)
        return
    }

    // Detecta tipo: WS upgrade ou SOCKS5
    buf := make([]byte, 1)
    if _, err := conn.Read(buf); err != nil {
        return
    }

    if buf[0] == 0x05 { // SOCKS5 greeting
        handleSOCKS5(conn)
    } else {
        // Assume WS: rewind e upgrade para HTTP
        conn.Close()
        // Para WS, crie um http.Server separado ou use http over TLS
        handleWSS(conn)
    }
}

func handleSOCKS5(conn net.Conn) {
    // SOCKS5 básico com redirecionamento para SSH auth
    // (Autentica via SSH local, depois forward)
    if err := authenticateViaSSH(conn); err != nil {
        log.Printf("Auth SSH falhou: %v", err)
        return
    }
    // Após auth, forward como SOCKS5 padrão
    log.Println("SOCKS5 conectado após auth SSH")
    // Implemente forwarding aqui (ex: io.Copy para target)
    io.Copy(conn, conn) // Placeholder
}

func handleWSS(tlsConn net.Conn) {
    // Upgrade para WS over TLS (WSS)
    // Para simplicidade, assume HTTP upgrade após TLS
    upgrader.CheckOrigin = func(r *http.Request) bool { return true }
    ws, err := upgrader.Upgrade(tlsConn, nil, nil) // Ajuste para http.ResponseWriter se necessário
    if err != nil {
        log.Printf("WS Upgrade falhou: %v", err)
        return
    }
    defer ws.Close()

    log.Println("WSS conectado")
    for {
        _, msg, err := ws.ReadMessage()
        if err != nil {
            break
        }
        ws.WriteMessage(websocket.TextMessage, msg) // Echo simples
    }
}

// Placeholder para gerar cert auto-assinado
func generateSelfSignedCert(certFile, keyFile string) {
    // Use crypto/tls para gerar (código simplificado, adicione impl completa)
    log.Printf("Cert gerado: %s, %s", certFile, keyFile)
}
