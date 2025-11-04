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

func clearScreen() {
    cmd := exec.Command("clear")
    cmd.Stdout = os.Stdout
    cmd.Run()
}

func RunMenu() {
    scanner := bufio.NewScanner(os.Stdin)
    for {
        clearScreen() // LIMPA A TELA A CADA ITERAÇÃO
        fmt.Println("╔════════════════════════════════════╗")
        fmt.Println("║        PROXY GO 2.0 - MENU         ║")
        fmt.Println("╚════════════════════════════════════╝")
        fmt.Println("")
        fmt.Println("1. Abrir porta")
        fmt.Println("2. Fechar porta")
        fmt.Println("3. Ver logs da porta")
        fmt.Println("4. Listar portas ativas")
        fmt.Println("5. Sair")
        fmt.Print("\nEscolha uma opção: ")

        if !scanner.Scan() {
            break
        }
        choice := strings.TrimSpace(scanner.Text())

        switch choice {
        case "1":
            clearScreen()
            fmt.Print("Digite a porta para abrir: ")
            scanner.Scan()
            port, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if err != nil || port < 1 || port > 65535 {
                fmt.Println("Porta inválida!")
                time.Sleep(2)
                continue
            }
            if err := StartBackgroundProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Proxy iniciado na porta %d (PID: %d)\n", port, instances[port].PID)
            }
            fmt.Print("\nPressione ENTER para continuar...")
            scanner.Scan()

        case "2":
            clearScreen()
            fmt.Print("Digite a porta para fechar: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if err := StopProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Porta %d fechada com sucesso.\n", port)
            }
            fmt.Print("\nPressione ENTER para continuar...")
            scanner.Scan()

        case "3":
            clearScreen()
            fmt.Print("Digite a porta para ver logs: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            ViewLogs(port)
            fmt.Print("\nPressione ENTER para voltar...")
            scanner.Scan()

        case "4":
            clearScreen()
            ListActivePorts()
            fmt.Print("\nPressione ENTER para voltar...")
            scanner.Scan()

        case "5":
            clearScreen()
            fmt.Println("Saindo do Proxy Go 2.0...")
            time.Sleep(1)
            return

        default:
            clearScreen()
            fmt.Println("Opção inválida! Tente novamente.")
            time.Sleep(2)
        }
    }
}

func StartBackgroundProxy(port int) error {
    mu.Lock()
    if _, exists := instances[port]; exists {
        mu.Unlock()
        return fmt.Errorf("porta %d já está em uso", port)
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

    fmt.Printf("Logs da porta %d (últimos 15):\n\n", port)
    start := len(logs) - 15
    if start < 0 {
        start = 0
    }
    if len(logs) == 0 {
        fmt.Println("Nenhum log ainda.")
    } else {
        for i := start; i < len(logs); i++ {
            fmt.Println("→", logs[i])
        }
    }
}

func ListActivePorts() {
    mu.Lock()
    defer mu.Unlock()

    if len(instances) == 0 {
        fmt.Println("Nenhuma porta ativa no momento.")
        return
    }

    fmt.Println("PORTAS ATIVAS:")
    fmt.Println("┌────────┬────────┬─────────┐")
    fmt.Printf("│ %-6s │ %-6s │ %-7s │\n", "PORTA", "PID", "STATUS")
    fmt.Println("├────────┼────────┼─────────┤")
    for port, inst := range instances {
        status := "RUNNING"
        if !inst.Running {
            status = "STOPPED"
        }
        fmt.Printf("│ %-6d │ %-6d │ %-7s │\n", port, inst.PID, status)
    }
    fmt.Println("└────────┴────────┴─────────┘")
}

func captureLogs(port int, inst *ProxyInstance) {
    ticker := time.NewTicker(3 * time.Second)
    for range ticker.C {
        mu.Lock()
        if inst == nil || !instances[port].Running {
            mu.Unlock()
            break
        }
        mu.Unlock()

        logEntry := fmt.Sprintf("%s - Conexão simulada na porta %d", time.Now().Format("15:04:05"), port)
        inst.Mutex.Lock()
        inst.Logs = append(inst.Logs, logEntry)
        if len(inst.Logs) > 100 {
            inst.Logs = inst.Logs[1:]
        }
        inst.Mutex.Unlock()
    }
}
