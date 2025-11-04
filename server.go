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
    "time" // <-- ADICIONADO

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

func StartServer(port int) error {
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

    log.Printf("Proxy rodando na porta %d (WSS + SOCKS5)\n", port)

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Println("Accept error:", err)
            continue
        }
        go handleClient(conn, tlsConfig.Clone())
    }
}

func handleClient(rawConn net.Conn, tlsConfig *tls.Config) {
    defer rawConn.Close()

    tlsConn := tls.Server(rawConn, tlsConfig)
    if err := tlsConn.Handshake(); err != nil {
        log.Println("TLS handshake failed:", err)
        return
    }
    defer tlsConn.Close()

    // Timeout para detectar protocolo
    tlsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
    buf := make([]byte, 1)
    n, err := tlsConn.Read(buf)
    if err != nil || n == 0 {
        handleWSS(tlsConn)
        return
    }

    if buf[0] == 0x05 {
        handleSOCKS5(tlsConn)
    } else {
        handleWSS(tlsConn)
    }
}

func handleSOCKS5(conn net.Conn) {
    log.Println("SOCKS5 detectado – autenticando via SSH...")
    if err := authenticateWithSSH(conn); err != nil {
        log.Println("SSH auth falhou:", err)
        return
    }
    log.Println("SOCKS5 autenticado com sucesso")
    // Forwarding real aqui (ex: io.Copy)
}

func handleWSS(conn net.Conn) {
    // Cria servidor HTTP temporário para upgrade
    server := &http.Server{
        Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if r.Header.Get("Upgrade") != "websocket" {
                http.Error(w, "WebSocket only", http.StatusBadRequest)
                return
            }
            ws, err := upgrader.Upgrade(w, r, nil)
            if err != nil {
                log.Println("WS upgrade error:", err)
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

    // Usa hijack para pegar conexão raw
    if hijacker, ok := conn.(http.Hijacker); ok {
        raw, _, _ := hijacker.Hijack()
        defer raw.Close()

        // Simula resposta HTTP 101
        fmt.Fprintf(raw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
            computeAcceptKey(r.Header.Get("Sec-WebSocket-Key")))

        // Aqui o WebSocket está ativo
        // (upgrader.Upgrade já foi chamado internamente)
    } else {
        log.Println("Conexão não suporta hijack")
    }
}

// Função auxiliar para WebSocket handshake
func computeAcceptKey(key string) string {
    h := sha1.New()
    h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func generateCert(certFile, keyFile string) error {
    cmd := exec.Command("openssl", "req", "-new", "-newkey", "rsa:2048", "-days", "365",
        "-nodes", "-x509", "-keyout", keyFile, "-out", certFile,
        "-subj", "/CN=localhost")
    return cmd.Run()
}

func authenticateWithSSH(conn net.Conn) error {
    cmd := exec.Command("ssh", "-o", "BatchMode=yes", "localhost", "true")
    cmd.Stdin = conn
    cmd.Stdout = conn
    return cmd.Run()
}
