// server.go
package main

import (
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "strings"
    "time"
)

func StartServer(port int) error {
    logFile := fmt.Sprintf("/var/log/proxy-go2.0-%d.log", port)
    f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    log.SetOutput(f)
    log.SetFlags(log.Ldate | log.Ltime)
    log.Printf("PROXY-GO2.0 ESCUTANDO NA PORTA %d", port)

    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil { return err }

    for {
        conn, err := listener.Accept()
        if err != nil { continue }
        go handleClient(conn)
    }
}

func handleClient(conn net.Conn) {
    defer conn.Close()

    // === RESPOSTA IMEDIATA PARA HTTP INJECTOR ===
    conn.Write([]byte("HTTP/1.1 200 PROXY-GO2.0\r\n\r\n"))
    log.Println("Resposta imediata enviada")

    buf := make([]byte, 1024)
    conn.SetReadDeadline(time.Now().Add(10 * time.Second))
    n, err := conn.Read(buf)
    if err != nil { return }

    data := string(buf[:n])
    log.Printf("Recebido: %s", strings.TrimSpace(data))

    // === DETECÇÃO DE PROTOCOLO ===
    if strings.Contains(data, "CONNECT") || strings.Contains(data, "Upgrade: websocket") {
        handleHTTPTunnel(conn)
    } else if buf[0] == 0x05 {
        handleSOCKS5(conn)
    } else {
        // Echo para qualquer coisa
        io.Copy(conn, conn)
    }
}

func handleHTTPTunnel(client net.Conn) {
    log.Println("Túnel HTTP/CONNECT")
    io.Copy(client, client)
}

func handleSOCKS5(client net.Conn) {
    log.Println("SOCKS5 detectado")
    // Handshake
    client.Write([]byte{0x05, 0x00})
    io.Copy(client, client)
}
