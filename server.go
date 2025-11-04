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
    f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return err
    }
    log.SetOutput(f)
    log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
    log.Printf("PROXY-GO2.0 iniciado na porta %d", port)

    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        return err
    }

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Printf("Accept error: %v", err)
            continue
        }
        go handleClient(conn, port)
    }
}

func handleClient(conn net.Conn, port int) {
    defer conn.Close()

    buf := make([]byte, 1024)
    conn.SetReadDeadline(time.Now().Add(5 * time.Second))
    n, err := conn.Read(buf)
    if err != nil {
        return
    }

    data := string(buf[:n])

    // === DETECÇÃO DE PROTOCOLO ===
    if strings.Contains(strings.ToUpper(data), "CONNECT") || strings.Contains(data, "Upgrade: websocket") {
        sendResponse(conn, "HTTP/1.1 101 PROXY-GO2.0\r\n\r\n")
        handleHTTPTunnel(conn, data)
    } else if buf[0] == 0x05 {
        sendResponse(conn, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n")
        handleSOCKS5(conn)
    } else {
        sendResponse(conn, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n")
    }
}

func sendResponse(conn net.Conn, msg string) {
    conn.Write([]byte(msg))
    log.Printf("→ %s", strings.TrimSpace(msg))
}

func handleHTTPTunnel(client net.Conn, request string) {
    host := "httpbin.org:80"
    if strings.Contains(request, "CONNECT") {
        parts := strings.Fields(strings.Split(request, "\n")[0])
        if len(parts) > 1 {
            host = parts[1]
        }
    }

    log.Printf("Túnel → %s", host)
    remote, err := net.Dial("tcp", host)
    if err != nil {
        log.Printf("Falha: %v", err)
        return
    }
    defer remote.Close()

    go io.Copy(remote, client)
    io.Copy(client, remote)
}

func handleSOCKS5(client net.Conn) {
    log.Println("SOCKS5 iniciado")

    buf := make([]byte, 3)
    if _, err := io.ReadFull(client, buf); err != nil || buf[0] != 0x05 {
        return
    }
    client.Write([]byte{0x05, 0x00})

    buf = make([]byte, 10)
    if _, err := io.ReadFull(client, buf); err != nil {
        return
    }

    var addr string
    switch buf[3] {
    case 0x01:
        addr = net.IP(buf[4:8]).String()
    case 0x03:
        l := int(buf[4])
        addr = string(buf[5 : 5+l])
    default:
        client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
        return
    }
    port := int(buf[len(buf)-2])<<8 + int(buf[len(buf)-1])
    target := fmt.Sprintf("%s:%d", addr, port)

    log.Printf("SOCKS5 → %s", target)
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
