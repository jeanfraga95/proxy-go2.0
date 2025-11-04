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
    PID     int
    Logs    []string
    Mutex   sync.Mutex
    Running bool
}

var (
    instances = make(map[int]*ProxyInstance)
    mu        sync.Mutex
)

func clearScreen() {
    exec.Command("clear").Run()
}

func RunMenu() {
    scanner := bufio.NewScanner(os.Stdin)
    for {
        clearScreen()
        fmt.Println("╔════════════════════════════════════╗")
        fmt.Println("║        PROXY GO 2.0 - TUNNEL       ║")
        fmt.Println("╚════════════════════════════════════╝")
        fmt.Println("")
        fmt.Println("1. Abrir porta (SOCKS5 + WSS)")
        fmt.Println("2. Fechar porta")
        fmt.Println("3. Ver logs da porta")
        fmt.Println("4. Listar portas ativas")
        fmt.Println("5. Sair")
        fmt.Print("\nEscolha: ")

        if !scanner.Scan() {
            break
        }
        choice := strings.TrimSpace(scanner.Text())

        switch choice {
        case "1":
            clearScreen()
            fmt.Print("Porta (ex: 80): ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if port < 1 || port > 65535 {
                fmt.Println("Porta inválida!")
                time.Sleep(2)
                continue
            }
            if err := StartBackgroundProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Proxy rodando na porta %d (PID: %d)\n", port, instances[port].PID)
                fmt.Println("→ Continua em background mesmo se fechar o menu!")
            }
            fmt.Print("\nENTER para continuar...")
            scanner.Scan()

        case "2":
            clearScreen()
            fmt.Print("Porta para fechar: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if err := StopProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Porta %d fechada.\n", port)
            }
            fmt.Print("\nENTER...")
            scanner.Scan()

        case "3":
            clearScreen()
            fmt.Print("Porta para logs: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            ViewLogs(port)
            fmt.Print("\nENTER...")
            scanner.Scan()

        case "4":
            clearScreen()
            ListActivePorts()
            fmt.Print("\nENTER...")
            scanner.Scan()

        case "5":
            clearScreen()
            fmt.Println("Saindo... Portas ativas continuam rodando!")
            time.Sleep(1)
            return

        default:
            clearScreen()
            fmt.Println("Opção inválida!")
            time.Sleep(2)
        }
    }
}

// Inicia proxy em background com nohup + setsid
func StartBackgroundProxy(port int) error {
    mu.Lock()
    if _, exists := instances[port]; exists {
        mu.Unlock()
        return fmt.Errorf("porta %d já em uso", port)
    }
    mu.Unlock()

    // Usa nohup + setsid para total independência
    cmd := exec.Command("nohup", os.Args[0], "-port="+strconv.Itoa(port), ">/dev/null", "2>&1", "&")
    cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
    if err := cmd.Start(); err != nil {
        return err
    }

    pid := cmd.Process.Pid
    inst := &ProxyInstance{Port: port, PID: pid, Running: true}
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
    if !ok {
        return fmt.Errorf("porta %d não encontrada", port)
    }

    // Mata o processo
    exec.Command("kill", "-9", strconv.Itoa(inst.PID)).Run()
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
        fmt.Printf("Porta %d não ativa.\n", port)
        return
    }

    inst.Mutex.Lock()
    logs := inst.Logs
    inst.Mutex.Unlock()

    fmt.Printf("Logs da porta %d (PID: %d)\n\n", port, inst.PID)
    if len(logs) == 0 {
        fmt.Println("Nenhum log ainda.")
    } else {
        for i := len(logs) - 10; i < len(logs); i++ {
            if i >= 0 {
                fmt.Printf("→ %s\n", logs[i])
            }
        }
    }
}

func ListActivePorts() {
    mu.Lock()
    defer mu.Unlock()
    if len(instances) == 0 {
        fmt.Println("Nenhuma porta ativa.")
        return
    }
    fmt.Println("PORTAS EM BACKGROUND:")
    fmt.Println("┌───────┬───────┬─────────┐")
    fmt.Printf("│ %-5s │ %-5s │ %-7s │\n", "PORTA", "PID", "STATUS")
    fmt.Println("├───────┼───────┼─────────┤")
    for port, inst := range instances {
        status := "ON"
        if !inst.Running {
            status = "OFF"
        }
        fmt.Printf("│ %-5d │ %-5d │ %-7s │\n", port, inst.PID, status)
    }
    fmt.Println("└───────┴───────┴─────────┘")
}

func captureLogs(port int, inst *ProxyInstance) {
    ticker := time.NewTicker(4 * time.Second)
    for range ticker.C {
        mu.Lock()
        running := instances[port].Running
        mu.Unlock()
        if !running {
            break
        }
        logEntry := fmt.Sprintf("%s - Túnel ativo na porta %d", time.Now().Format("15:04:05"), port)
        inst.Mutex.Lock()
        inst.Logs = append(inst.Logs, logEntry)
        if len(inst.Logs) > 50 {
            inst.Logs = inst.Logs[1:]
        }
        inst.Mutex.Unlock()
    }
}
