// proxy.go
package main

import (
    "bufio"
    "encoding/json"
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
    Port int  `json:"port"`
    PID  int  `json:"pid"`
}

type State struct {
    Instances map[int]*ProxyInstance `json:"instances"`
}

var (
    stateFile = "/opt/proxy-go2.0/ports.json"
    mu        sync.Mutex
)

func clearScreen() {
    exec.Command("clear").Run()
}

func loadState() map[int]*ProxyInstance {
    mu.Lock()
    defer mu.Unlock()

    data, err := os.ReadFile(stateFile)
    if err != nil {
        return make(map[int]*ProxyInstance)
    }

    var state State
    if json.Unmarshal(data, &state) != nil {
        return make(map[int]*ProxyInstance)
    }
    return state.Instances
}

func saveState(instances map[int]*ProxyInstance) {
    mu.Lock()
    defer mu.Unlock()

    state := State{Instances: instances}
    data, _ := json.MarshalIndent(state, "", "  ")
    os.WriteFile(stateFile, data, 0644)
}

func RunMenu() {
    instances := loadState()
    scanner := bufio.NewScanner(os.Stdin)

    for {
        clearScreen()
        fmt.Println("╔════════════════════════════════════╗")
        fmt.Println("║        PROXY GO 2.0 - TUNNEL       ║")
        fmt.Println("╚════════════════════════════════════╝")
        fmt.Println("")
        fmt.Println("1. Abrir porta")
        fmt.Println("2. Fechar porta")
        fmt.Println("3. Ver logs")
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
            fmt.Print("Porta: ")
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
                fmt.Printf("Proxy iniciado na porta %d\n", port)
            }
            fmt.Print("\nENTER...")
            scanner.Scan()

        case "2":
            clearScreen()
            fmt.Print("Porta para fechar: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if err := StopProxy(port, &instances); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Porta %d fechada.\n", port)
            }
            saveState(instances)
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
            ListActivePorts(instances)
            fmt.Print("\nENTER...")
            scanner.Scan()

        case "5":
            clearScreen()
            fmt.Println("Saindo... Portas continuam ativas!")
            time.Sleep(1)
            return

        default:
            clearScreen()
            fmt.Println("Opção inválida!")
            time.Sleep(2)
        }
    }
}

func StartBackgroundProxy(port int) error {
    // Verifica se já existe
    instances := loadState()
    if _, exists := instances[port]; exists {
        return fmt.Errorf("porta %d já em uso", port)
    }

    // Inicia processo independente
    cmd := exec.Command("nohup", os.Args[0], "-port="+strconv.Itoa(port), ">/dev/null", "2>&1", "&")
    cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
    if err := cmd.Start(); err != nil {
        return err
    }

    inst := &ProxyInstance{Port: port, PID: cmd.Process.Pid}
    instances[port] = inst
    saveState(instances)
    return nil
}

func StopProxy(port int, instances *map[int]*ProxyInstance) error {
    inst, ok := (*instances)[port]
    if !ok {
        return fmt.Errorf("porta %d não encontrada", port)
    }

    // Mata processo
    proc, err := os.FindProcess(inst.PID)
    if err != nil {
        delete(*instances, port)
        return nil
    }
    proc.Signal(syscall.SIGKILL)
    delete(*instances, port)
    return nil
}

func ViewLogs(port int) {
    logFile := fmt.Sprintf("/var/log/proxy-go2.0-%d.log", port)
    data, _ := os.ReadFile(logFile)
    lines := strings.Split(string(data), "\n")
    start := len(lines) - 15
    if start < 0 {
        start = 0
    }
    fmt.Printf("Logs da porta %d:\n\n", port)
    for i := start; i < len(lines); i++ {
        if lines[i] != "" {
            fmt.Println("→", lines[i])
        }
    }
}

func ListActivePorts(instances map[int]*ProxyInstance) {
    if len(instances) == 0 {
        fmt.Println("Nenhuma porta ativa.")
        return
    }
    fmt.Println("PORTAS ATIVAS (persistentes):")
    fmt.Println("┌───────┬───────┐")
    fmt.Printf("│ %-5s │ %-5s │\n", "PORTA", "PID")
    fmt.Println("├───────┼───────┤")
    for port, inst := range instances {
        fmt.Printf("│ %-5d │ %-5d │\n", port, inst.PID)
    }
    fmt.Println("└───────┴───────┘")
}
