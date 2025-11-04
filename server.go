// server.go
package main

import (
    "crypto/tls"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "os/exec"
    "strings"
    "time"
)

func StartServer(port int) error {
    // === GERA CERTIFICADO TLS ===
    certFile := fmt.Sprintf("/tmp/cert-%d.pem", port)
    keyFile := fmt.Sprintf("/tmp/key-%d.pem", port)

    if _, err := os.Stat(certFile); os.IsNotExist(err) {
        generateCert(certFile, keyFile)
    }

    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return err
    }

    // === LOG POR PORTA ===
    logFile := fmt.Sprintf("/var/log/proxy-go2.0-%d.log", port)
    f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    log.SetOutput(f)
    log.SetFlags(log.Ldate | log.Ltime)
    log.Printf("PROXY-GO2.0 INICIADO NA PORTA %d (WSS + SOCKS5)", port)

    // === LISTENER TLS ===
    tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
    listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), tlsConfig)
    if err != nil {
        return err
    }

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Printf("Accept error: %v", err)
            continue
        }
        go handleClient(conn)
    }
}

func handleClient(rawConn net.Conn) {
    defer rawConn.Close()

    tlsConn, ok := rawConn.(*tls.Conn)
    if !ok {
        return
    }
    if err := tlsConn.Handshake(); err != nil {
        return
    }

    // === RESPOSTA IMEDIATA PARA HTTP INJECTOR ===
    tlsConn.Write([]byte("HTTP/1.1 200 PROXY-GO2.0\r\n\r\n"))
    log.Println("Resposta imediata: HTTP/1.1 200 PROXY-GO2.0")

    // === LÊ PRIMEIROS BYTES ===
    buf := make([]byte, 1024)
    tlsConn.SetReadDeadline(time.Now().Add(10 * time.Second))
    n, err := tlsConn.Read(buf)
    if err != nil {
        return
    }

    data := string(buf[:n])
    log.Printf("Recebido: %s", strings.TrimSpace(data))

    // === DETECÇÃO DE PROTOCOLO ===
    if strings.Contains(data, "Upgrade: websocket") {
        handleWSS(tlsConn)
    } else if buf[0] == 0x05 {
        handleSOCKS5(tlsConn)
    } else if strings.Contains(data, "CONNECT") {
        handleHTTPTunnel(tlsConn)
    } else {
        io.Copy(tlsConn, tlsConn)
    }
}

// === WSS (WebSocket Secure) ===
func handleWSS(conn net.Conn) {
    log.Println("WSS detectado - Túnel ativo")
    io.Copy(conn, conn) // Echo ou forwarding
}

// === SOCKS5 (HTTP Injector) ===
func handleSOCKS5(conn net.Conn) {
    log.Println("SOCKS5 detectado")
    conn.Write([]byte{0x05, 0x00}) // No auth
    io.Copy(conn, conn)
}

// === HTTP CONNECT (Tunnel) ===
func handleHTTPTunnel(conn net.Conn) {
    log.Println("HTTP CONNECT detectado")
    io.Copy(conn, conn)
}

// === GERA CERTIFICADO AUTO-ASSINADO ===
func generateCert(certFile, keyFile string) {
    exec.Command("openssl", "req", "-new", "-newkey", "rsa:2048", "-days", "365",
        "-nodes", "-x509", "-keyout", keyFile, "-out", certFile,
        "-subj", "/CN=localhost").Run()
}
