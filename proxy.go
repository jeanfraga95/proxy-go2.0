// proxy.go
package main

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "sync"
    "syscall"
    "time"
)

type ProxyInstance struct {
    Port    int
    Cmd     *exec.Cmd
    PID     int
    Logs    []string
    Mutex   sync.Mutex
    Running bool
}

var (
    instances = make(map[int]*ProxyInstance)
    mu        sync.Mutex
)

func RunMenu() {
    scanner := bufio.NewScanner(os.Stdin)
    for {
        fmt.Println("\n--- Menu Proxy ---")
        fmt.Println("1. Abrir porta")
        fmt.Println("2. Fechar porta")
        fmt.Println("3. Ver logs")
        fmt.Println("4. Listar portas")
        fmt.Println("5. Sair")
        fmt.Print("Escolha: ")

        if !scanner.Scan() {
            break
        }
        choice := strings.TrimSpace(scanner.Text())

        switch choice {
        case "1":
            fmt.Print("Porta: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if err := StartBackgroundProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Proxy iniciado na porta %d\n", port)
            }

        case "2":
            fmt.Print("Porta: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if err := StopProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Porta %d fechada.\n", port)
            }

        case "3":
            fmt.Print("Porta: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            ViewLogs(port)

        case "4":
            ListActivePorts()

        case "5":
            fmt.Println("Saindo...")
            return

        default:
            fmt.Println("Opção inválida.")
        }
    }
}

func StartBackgroundProxy(port int) error {
    mu.Lock()
    if _, exists := instances[port]; exists {
        mu.Unlock()
        return fmt.Errorf("porta %d já em uso", port)
    }
    mu.Unlock()

    cmd := exec.Command(os.Args[0], "-port="+strconv.Itoa(port))
    cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
    if err := cmd.Start(); err != nil {
        return err
    }

    inst := &ProxyInstance{
        Port:    port,
        Cmd:     cmd,
        PID:     cmd.Process.Pid,
        Running: true,
    }

    mu.Lock()
    instances[port] = inst
    mu.Unlock()

    go captureLogs(port, inst)
    return nil
}

func StopProxy(port int) error {
    mu.Lock()
    inst, ok := instances[port]
    mu.Unlock()
    if !ok || !inst.Running {
        return fmt.Errorf("porta %d não está ativa", port)
    }

    inst.Cmd.Process.Signal(syscall.SIGTERM)
    inst.Running = false
    mu.Lock()
    delete(instances, port)
    mu.Unlock()
    return nil
}

func ViewLogs(port int) {
    mu.Lock()
    inst, ok := instances[port]
    mu.Unlock()
    if !ok {
        fmt.Printf("Porta %d não encontrada.\n", port)
        return
    }

    inst.Mutex.Lock()
    logs := inst.Logs
    inst.Mutex.Unlock()

    fmt.Printf("=== Logs da porta %d (últimos 15) ===\n", port)
    start := len(logs) - 15
    if start < 0 {
        start = 0
    }
    for i := start; i < len(logs); i++ {
        fmt.Println(logs[i])
    }
}

func ListActivePorts() {
    mu.Lock()
    defer mu.Unlock()
    if len(instances) == 0 {
        fmt.Println("Nenhuma porta ativa.")
        return
    }
    fmt.Println("Portas ativas:")
    for port, inst := range instances {
        status := "RUNNING"
        if !inst.Running {
            status = "STOPPED"
        }
        fmt.Printf("  • %d (PID: %d) → %s\n", port, inst.PID, status)
    }
}

func captureLogs(port int, inst *ProxyInstance) {
    ticker := time.NewTicker(3 * time.Second)
    for range ticker.C {
        mu.Lock()
        if !instances[port].Running {
            mu.Unlock()
            break
        }
        mu.Unlock()

        logEntry := fmt.Sprintf("[%s] Porta %d: conexão simulada", time.Now().Format("15:04:05"), port)
        inst.Mutex.Lock()
        inst.Logs = append(inst.Logs, logEntry)
        if len(inst.Logs) > 100 {
            inst.Logs = inst.Logs[1:]
        }
        inst.Mutex.Unlock()
    }
}
