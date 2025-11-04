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

    log.Printf("PROXY-GO2.0 rodando na porta %d (SOCKS5 + WSS Tunnel)", port)

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
        log.Println("TLS handshake falhou:", err)
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
    if strings.Contains(strings.ToUpper(data), "UPGRADE: WEBSOCKET") || strings.Contains(data, "CONNECT") {
        sendResponse(tlsConn, "HTTP/1.1 101 PROXY-GO2.0\r\n\r\n")
        handleHTTPTunnel(tlsConn, data)
    } else if buf[0] == 0x05 {
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

// === TUNNEL HTTP (CONNECT / WSS) ===
func handleHTTPTunnel(client net.Conn, request string) {
    var host string
    lines := strings.Split(request, "\n")
    for _, line := range lines {
        if strings.HasPrefix(strings.TrimSpace(line), "CONNECT") {
            host = strings.Fields(line)[1]
            break
        }
    }
    if host == "" {
        host = "httpbin.org:80" // fallback
    }

    log.Printf("Túnel HTTP para %s", host)

    remote, err := net.Dial("tcp", host)
    if err != nil {
        log.Println("Falha ao conectar ao destino:", err)
        return
    }
    defer remote.Close()

    // Bidirectional copy
    go io.Copy(remote, client)
    io.Copy(client, remote)
}

// === SOCKS5 REAL (com forwarding) ===
func handleSOCKS5(client net.Conn) {
    log.Println("SOCKS5 iniciado")

    // Handshake SOCKS5
    buf := make([]byte, 3)
    if _, err := io.ReadFull(client, buf); err != nil || buf[0] != 0x05 {
        return
    }
    client.Write([]byte{0x05, 0x00}) // No auth

    // Request
    buf = make([]byte, 10)
    if _, err := io.ReadFull(client, buf); err != nil {
        return
    }

    var addr string
    switch buf[3] {
    case 0x01: // IPv4
        addr = net.IP(buf[4:8]).String()
    case 0x03: // Domain
        len := int(buf[4])
        addr = string(buf[5 : 5+len])
    default:
        client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
        return
    }
    port := int(buf[len(buf)-2])<<8 + int(buf[len(buf)-1])
    target := fmt.Sprintf("%s:%d", addr, port)

    log.Printf("SOCKS5 → Conectando a %s", target)

    remote, err := net.Dial("tcp", target)
    if err != nil {
        client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
        return
    }
    defer remote.Close()

    client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

    go io.Copy(remote, client)
    io.Copy(client, remote)
}

func generateCert(certFile, keyFile string) {
    exec.Command("openssl", "req", "-new", "-newkey", "rsa:2048", "-days", "365",
        "-nodes", "-x509", "-keyout", keyFile, "-out", certFile,
        "-subj", "/CN=proxy-go2.0").Run()
}

func authenticateWithSSH(conn net.Conn) {
    // Placeholder: autenticação via SSH local (opcional)
    // Por enquanto, ignora (ou use usuário/senha depois)
}
