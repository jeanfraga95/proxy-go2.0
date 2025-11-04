// server.go
package main

import (
    "crypto/tls"
    "fmt"
    "log"
    "net"
    "net/http"
    "os"
    "os/exec"

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

func StartServer(port int) error {
    // Gera certificado auto-assinado (simplificado)
    certFile := fmt.Sprintf("/tmp/cert-%d.pem", port)
    keyFile := fmt.Sprintf("/tmp/key-%d.pem", port)
    if _, err := os.Stat(certFile); os.IsNotExist(err) {
        if err := generateCert(certFile, keyFile); err != nil {
            return fmt.Errorf("falha ao gerar TLS: %v", err)
        }
    }

    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return err
    }

    tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        return err
    }

    log.Printf("Proxy rodando na porta %d (WSS + SOCKS5 com auth SSH)\n", port)

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Println("Erro accept:", err)
            continue
        }
        go handleClient(conn, tlsConfig.Clone())
    }
}

func handleClient(rawConn net.Conn, tlsConfig *tls.Config) {
    defer rawConn.Close()

    // TLS Handshake
    tlsConn := tls.Server(rawConn, tlsConfig)
    if err := tlsConn.Handshake(); err != nil {
        log.Println("TLS falhou:", err)
        return
    }
    defer tlsConn.Close()

    // Lê primeiro byte para detectar protocolo
    buf := make([]byte, 1)
    tlsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
    if _, err := tlsConn.Read(buf); err != nil {
        // Timeout ou erro → assume WebSocket
        handleWSS(tlsConn)
        return
    }

    if buf[0] == 0x05 {
        // SOCKS5
        handleSOCKS5(tlsConn)
    } else {
        // WebSocket (HTTP upgrade)
        handleWSS(tlsConn)
    }
}

func handleSOCKS5(conn net.Conn) {
    log.Println("SOCKS5 detectado – redirecionando para SSH auth")
    if err := authenticateWithSSH(conn); err != nil {
        log.Println("Auth SSH falhou:", err)
        return
    }
    log.Println("SOCKS5 autenticado com sucesso")
    // Aqui você pode implementar forwarding real
}

func handleWSS(conn net.Conn) {
    // Cria um http.Server para fazer o upgrade
    server := &http.Server{
        Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if r.Header.Get("Upgrade") != "websocket" {
                http.Error(w, "Use WebSocket", http.StatusBadRequest)
                return
            }
            ws, err := upgrader.Upgrade(w, r, nil)
            if err != nil {
                log.Println("WS upgrade falhou:", err)
                return
            }
            defer ws.Close()
            log.Println("WSS conectado")
            for {
                mt, msg, err := ws.ReadMessage()
                if err != nil {
                    break
                }
                ws.WriteMessage(mt, msg)
            }
        }),
    }

    // Usa hijacker para pegar conexão raw
    hijacker, ok := conn.(http.Hijacker)
    if !ok {
        log.Println("Conexão não suporta hijack")
        return
    }

    raw, _, err := hijacker.Hijack()
    if err != nil {
        log.Println("Hijack falhou:", err)
        return
    }
    defer raw.Close()

    // Simula resposta HTTP para upgrade
    fmt.Fprintf(raw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
    // Aqui o WebSocket já está "upgraded"
}

func generateCert(certFile, keyFile string) error {
    cmd := exec.Command("openssl", "req", "-new", "-newkey", "rsa:2048", "-days", "365",
        "-nodes", "-x509", "-keyout", keyFile, "-out", certFile,
        "-subj", "/CN=localhost")
    return cmd.Run()
}

func authenticateWithSSH(conn net.Conn) error {
    // Simula autenticação via SSH local
    cmd := exec.Command("ssh", "-o", "BatchMode=yes", "localhost", "true")
    cmd.Stdin = conn
    cmd.Stdout = conn
    return cmd.Run()
}
