// server.go
package main

import (
    "crypto/sha1"
    "encoding/base64"
    "crypto/tls"
    "fmt"
    "log"
    "net"
    "net/http"
    "os"
    "os/exec"
    "time"

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

// Inicia o proxy na porta (WSS + SOCKS5)
func StartServer(port int) error {
    certFile := fmt.Sprintf("/tmp/cert-%d.pem", port)
    keyFile := fmt.Sprintf("/tmp/key-%d.pem", port)

    if _, err := os.Stat(certFile); os.IsNotExist(err) {
        if err := generateCert(certFile, keyFile); err != nil {
            return fmt.Errorf("falha ao gerar certificado: %v", err)
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

    log.Printf("Proxy rodando na porta %d (WSS + SOCKS5 com SSH auth)\n", port)

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

    tlsConn := tls.Server(rawConn, tlsConfig)
    if err := tlsConn.Handshake(); err != nil {
        log.Println("TLS handshake falhou:", err)
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
    log.Println("SOCKS5 detectado – autenticando via OpenSSH...")
    if err := authenticateWithSSH(conn); err != nil {
        log.Println("Falha na autenticação SSH:", err)
        return
    }
    log.Println("SOCKS5 autenticado com sucesso")
    // Aqui você pode adicionar forwarding real
}

func handleWSS(conn net.Conn) {
    // Cria handler HTTP para upgrade
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Upgrade") != "websocket" {
            http.Error(w, "Use WebSocket", http.StatusBadRequest)
            return
        }

        // Calcula Sec-WebSocket-Accept
        key := r.Header.Get("Sec-WebSocket-Key")
        accept := computeAcceptKey(key)

        // Responde handshake manualmente
        w.Header().Set("Upgrade", "websocket")
        w.Header().Set("Connection", "Upgrade")
        w.Header().Set("Sec-WebSocket-Accept", accept)
        w.WriteHeader(http.StatusSwitchingProtocols)

        // Hijack para controle total
        hijacker, ok := w.(http.Hijacker)
        if !ok {
            log.Println("Hijack não suportado")
            return
        }

        clientConn, _, err := hijacker.Hijack()
        if err != nil {
            log.Println("Hijack falhou:", err)
            return
        }
        defer clientConn.Close()

        // WebSocket conectado
        log.Println("WSS conectado com sucesso")
        // Echo simples
        buf := make([]byte, 1024)
        for {
            n, err := clientConn.Read(buf)
            if err != nil {
                break
            }
            clientConn.Write(buf[:n])
        }
    })

    // Servidor HTTP temporário
    server := &http.Server{Handler: handler}
    server.ServeTLS(&singleConnListener{conn}, "", "")
}

type singleConnListener struct {
    conn net.Conn
}

func (l *singleConnListener) Accept() (net.Conn, error) {
    if l.conn == nil {
        return nil, fmt.Errorf("conexão fechada")
    }
    c := l.conn
    l.conn = nil
    return c, nil
}

func (l *singleConnListener) Close() error { return nil }
func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

// Handshake WebSocket
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
    cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", "localhost", "true")
    cmd.Stdin = conn
    cmd.Stdout = conn
    return cmd.Run()
}
