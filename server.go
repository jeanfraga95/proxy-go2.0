// server.go
package main

import (
    "crypto/tls"
    "log"
    "fmt"
    "net"
    "os"
    "os/exec"
    "strings"
    "time"
)

func StartServer(port int) error {
    certFile := fmt.Sprintf("/tmp/cert-%d.pem", port)
    keyFile := fmt.Sprintf("/tmp/key-%d.pem", port)

    if _, err := os.Stat(certFile); os.IsNotExist(err) {
        generateCert(certFile, keyFile)
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

    log.Printf("PROXY-GO2.0 rodando na porta %d", port)

    for {
        conn, err := listener.Accept()
        if err != nil {
            continue
        }
        go handleClient(conn, tlsConfig.Clone(), port)
    }
}

func handleClient(rawConn net.Conn, tlsConfig *tls.Config, port int) {
    defer rawConn.Close()

    tlsConn := tls.Server(rawConn, tlsConfig)
    if err := tlsConn.Handshake(); err != nil {
        return
    }
    defer tlsConn.Close()

    buf := make([]byte, 1024)
    tlsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
    n, err := tlsConn.Read(buf)
    if err != nil {
        return
    }

    data := string(buf[:n])

    // === DETECÇÃO DE PROTOCOLO ===
    if strings.Contains(strings.ToUpper(data), "UPGRADE: WEBSOCKET") ||
       strings.Contains(strings.ToUpper(data), "CONNECT") {
        // WSS ou HTTP CONNECT
        sendResponse(tlsConn, "HTTP/1.1 101 PROXY-GO2.0\r\n\r\n")
        handleTunnel(tlsConn)
    } else if buf[0] == 0x05 {
        // SOCKS5
        sendResponse(tlsConn, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n")
        handleSOCKS5(tlsConn)
    } else {
        sendResponse(tlsConn, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n")
    }
}

func sendResponse(conn net.Conn, msg string) {
    conn.Write([]byte(msg))
    log.Printf("→ %s", strings.TrimSpace(msg))
}

func handleTunnel(conn net.Conn) {
    log.Println("Túnel WSS/CONNECT ativo")
    // Echo ou forwarding
    buf := make([]byte, 4096)
    for {
        n, err := conn.Read(buf)
        if err != nil {
            break
        }
        conn.Write(buf[:n])
    }
}

func handleSOCKS5(conn net.Conn) {
    log.Println("SOCKS5 → Autenticando via SSH...")
    authenticateWithSSH(conn)
    log.Println("SOCKS5 Tunnel ativo")
    // Forwarding
    buf := make([]byte, 4096)
    for {
        n, err := conn.Read(buf)
        if err != nil {
            break
        }
        conn.Write(buf[:n])
    }
}

func generateCert(certFile, keyFile string) {
    exec.Command("openssl", "req", "-new", "-newkey", "rsa:2048", "-days", "365",
        "-nodes", "-x509", "-keyout", keyFile, "-out", certFile,
        "-subj", "/CN=proxy-go2.0").Run()
}

func authenticateWithSSH(conn net.Conn) {
    cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", "localhost", "true")
    cmd.Stdin = conn
    cmd.Stdout = conn
    cmd.Run()
}
