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
    // === GERA CERTIFICADO TLS (se não existir) ===
    certFile := fmt.Sprintf("/tmp/cert-%d.pem", port)
    keyFile := fmt.Sprintf("/tmp/key-%d.pem", port)

    if _, err := os.Stat(certFile); os.IsNotExist(err) {
        if err := generateCert(certFile, keyFile); err != nil {
            return fmt.Errorf("falha ao gerar certificado: %v", err)
        }
    }

    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return fmt.Errorf("erro ao carregar certificado: %v", err)
    }

    // === CONFIGURA LOG POR PORTA ===
    logFile := fmt.Sprintf("/var/log/proxy-go2.0-%d.log", port)
    f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return fmt.Errorf("erro ao abrir log: %v", err)
    }
    log.SetOutput(f)
    log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
    log.Printf("PROXY-GO2.0 iniciado na porta %d (SOCKS5 + WSS + CONNECT)", port)

    // === CONFIGURA LISTENER TLS ===
    tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        return fmt.Errorf("erro ao abrir porta %d: %v", port, err)
    }

    // === LOOP DE ACEITAÇÃO ===
    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Printf("Erro ao aceitar conexão: %v", err)
            continue
        }
        go handleClient(conn, tlsConfig.Clone(), port)
    }
}

func handleClient(rawConn net.Conn, tlsConfig *tls.Config, port int) {
    defer rawConn.Close()

    // === TLS HANDSHAKE ===
    tlsConn := tls.Server(rawConn, tlsConfig)
    if err := tlsConn.Handshake(); err != nil {
        log.Printf("TLS handshake falhou: %v", err)
        return
    }
    defer tlsConn.Close()

    // === LÊ PRIMEIROS BYTES ===
    buf := make([]byte, 1024)
    tlsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
    n, err := tlsConn.Read(buf)
    if err != nil {
        return
    }

    data := string(buf[:n])

    // === DETECÇÃO DE PROTOCOLO ===
    upperData := strings.ToUpper(data)

    if strings.Contains(upperData, "CONNECT") || strings.Contains(upperData, "UPGRADE: WEBSOCKET") {
        // HTTP CONNECT ou WSS
        sendResponse(tlsConn, "HTTP/1.1 101 PROXY-GO2.0\r\n\r\n")
        handleHTTPTunnel(tlsConn, data)
    } else if buf[0] == 0x05 {
        // SOCKS5
        sendResponse(tlsConn, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n")
        handleSOCKS5(tlsConn)
    } else {
        // Fallback
        sendResponse(tlsConn, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n")
    }
}

func sendResponse(conn net.Conn, msg string) {
    conn.Write([]byte(msg))
    log.Printf("→ Resposta: %s", strings.TrimSpace(msg))
}

// === TUNNEL HTTP (CONNECT / WSS) ===
func handleHTTPTunnel(client net.Conn, request string) {
    var host string
    lines := strings.Split(strings.TrimSpace(request), "\n")
    for _, line := range lines {
        if strings.HasPrefix(strings.TrimSpace(line), "CONNECT") {
            parts := strings.Fields(line)
            if len(parts) >= 2 {
                host = parts[1]
            }
            break
        }
    }
    if host == "" {
        host = "httpbin.org:80"
    }

    log.Printf("Túnel HTTP/CONNECT → %s", host)

    remote, err := net.Dial("tcp", host)
    if err != nil {
        log.Printf("Falha ao conectar ao destino %s: %v", host, err)
        return
    }
    defer remote.Close()

    go io.Copy(remote, client)
    io.Copy(client, remote)
}

// === SOCKS5 REAL (com forwarding) ===
func handleSOCKS5(client net.Conn) {
    log.Println("SOCKS5 iniciado")

    // Handshake: versão + métodos
    buf := make([]byte, 3)
    if _, err := io.ReadFull(client, buf); err != nil || buf[0] != 0x05 {
        log.Println("SOCKS5 handshake inválido")
        return
    }
    client.Write([]byte{0x05, 0x00}) // Sem autenticação

    // Request
    buf = make([]byte, 10)
    if _, err := io.ReadFull(client, buf); err != nil {
        return
    }

    var addr string
    switch buf[3] {
    case 0x01: // IPv4
        addr = net.IP(buf[4:8]).String()
    case 0x03: // Domínio
        len := int(buf[4])
        addr = string(buf[5 : 5+len])
    case 0x04: // IPv6
        addr = net.IP(buf[4:20]).String()
    default:
        client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
        return
    }
    port := int(buf[len(buf)-2])<<8 + int(buf[len(buf)-1])
    target := fmt.Sprintf("%s:%d", addr, port)

    log.Printf("SOCKS5 → Conectando a %s", target)

    remote, err := net.Dial("tcp", target)
    if err != nil {
        log.Printf("Falha ao conectar a %s: %v", target, err)
        client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
        return
    }
    defer remote.Close()

    client.Write([]byte{0x05, 0x00, 0x00, 0
