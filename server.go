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
    logFile := fmt.Sprintf("/var/log/proxy-go2.0-%d.log", port)
    f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    log.SetOutput(f)
    log.SetFlags(log.Ldate | log.Ltime)
    log.Printf("PROXY-GO2.0 HÍBRIDO INICIADO NA PORTA %d", port)

    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        return err
    }

    for {
        conn, err := listener.Accept()
        if err != nil {
            continue
        }
        go handleClient(conn, port)
    }
}

func handleClient(conn net.Conn, port int) {
    defer conn.Close()

    // === PEEK NOS PRIMEIROS BYTES (sem consumir) ===
    buf := make([]byte, 1)
    conn.SetReadDeadline(time.Now().Add(3 * time.Second))
    _, err := conn.Read(buf)
    if err != nil {
        return
    }

    firstByte := buf[0]

    // === DEVOLVE O BYTE PARA O STREAM ===
    peekConn := &peekedConn{conn, []byte{firstByte}}

    // === MODO HÍBRIDO ===
    if firstByte == 0x05 {
        // SOCKS5 → HTTP Injector (sem TLS)
        sendResponse(peekConn, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n")
        handleSOCKS5(peekConn)
    } else if firstByte == 0x16 {
        // TLS Handshake → WSS
        handleWSS(conn)
    } else {
        // HTTP/CONNECT ou outro
        sendResponse(peekConn, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n")
        handleHTTPTunnel(peekConn)
    }
}

func sendResponse(conn net.Conn, msg string) {
    conn.Write([]byte(msg))
    log.Printf("→ %s", strings.TrimSpace(msg))
}

// === SOCKS5 PURO (HTTP Injector) ===
func handleSOCKS5(conn net.Conn) {
    log.Println("SOCKS5 (HTTP Injector) detectado")
    buf := make([]byte, 255)
    conn.SetReadDeadline(time.Now().Add(10 * time.Second))
    n, _ := conn.Read(buf)
    if n > 0 && buf[0] == 0x05 {
        conn.Write([]byte{0x05, 0x00}) // No auth
    }
    io.Copy(conn, conn)
}

// === WSS (WebSocket Secure) ===
func handleWSS(rawConn net.Conn) {
    log.Println("WSS (TLS) detectado")
    certFile := fmt.Sprintf("/tmp/cert-%d.pem", 80) // ou porta dinâmica
    keyFile := fmt.Sprintf("/tmp/key-%d.pem", 80)
    if _, err := os.Stat(certFile); os.IsNotExist(err) {
        generateCert(certFile, keyFile)
    }
    cert, _ := tls.LoadX509KeyPair(certFile, keyFile)
    tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
    tlsConn := tls.Server(rawConn, tlsConfig)
    if err := tlsConn.Handshake(); err != nil {
        return
    }
    sendResponse(tlsConn, "HTTP/1.1 101 PROXY-GO2.0\r\n\r\n")
    io.Copy(tlsConn, tlsConn)
}

// === HTTP CONNECT ===
func handleHTTPTunnel(conn net.Conn) {
    log.Println("HTTP CONNECT detectado")
    io.Copy(conn, conn)
}

// === GERA CERTIFICADO ===
func generateCert(certFile, keyFile string) {
    exec.Command("openssl", "req", "-new", "-newkey", "rsa:2048", "-days", "365",
        "-nodes", "-x509", "-keyout", keyFile, "-out", certFile,
        "-subj", "/CN=localhost").Run()
}

// === CONN COM PEEK ===
type peekedConn struct {
    net.Conn
    peek []byte
}

func (p *peekedConn) Read(b []byte) (int, error) {
    if len(p.peek) > 0 {
        copy(b, p.peek)
        n := len(p.peek)
        p.peek = nil
        return n, nil
    }
    return p.Conn.Read(b)
}
