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
    Port int `json:"port"`
    PID  int `json:"pid"`
}

var (
    stateFile = "/opt/proxy-go2.0/ports.json"
    mu        sync.Mutex
)

// === UTILIDADES ===
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

    var state struct {
        Instances map[int]*ProxyInstance `json:"instances"`
    }
    if json.Unmarshal(data, &state) != nil || state.Instances == nil {
        return make(map[int]*ProxyInstance)
    }
    return state.Instances
}

func saveState(instances map[int]*ProxyInstance) {
    mu.Lock()
    defer mu.Unlock()

    data, _ := json.MarshalIndent(map[string]any{"instances": instances}, "", "  ")
    os.WriteFile(stateFile, data, 0644)
}

func waitEnter() {
    fmt.Print("\nPressione ENTER para continuar...")
    bufio.NewReader(os.Stdin).ReadString('\n')
}

// === MENU PRINCIPAL ===
func RunMenu() {
    scanner := bufio.NewScanner(os.Stdin)

    for {
        instances := loadState()
        clearScreen()
        fmt.Println("╔════════════════════════════════════╗")
        fmt.Println("║      PROXY GO 2.0 - TUNNEL         ║")
        fmt.Println("╚════════════════════════════════════╝\n")
        fmt.Println("1. Abrir porta (WSS + SOCKS5)")
        fmt.Println("2. Fechar porta")
        fmt.Println("3. Ver logs da porta")
        fmt.Println("4. Listar portas ativas")
        fmt.Println("5. Sair")
        fmt.Print("\nEscolha: ")

        if !scanner.Scan() {
            fmt.Println("\nSaindo...")
            time.Sleep(1)
            return
        }
        choice := strings.TrimSpace(scanner.Text())

        switch choice {
        case "1":
            clearScreen()
            fmt.Print("Digite a porta (ex: 80): ")
            if !scanner.Scan() {
                return
            }
            portStr := strings.TrimSpace(scanner.Text())
            port, err := strconv.Atoi(portStr)
            if err != nil || port < 1 || port > 65535 {
                fmt.Println("Porta inválida!")
                waitEnter()
                continue
            }
            if _, exists := instances[port]; exists {
                fmt.Printf("Porta %d já está em uso!\n", port)
            } else if err := StartBackgroundProxy(port); err != nil {
                fmt.Printf("Erro ao iniciar: %v\n", err)
            } else {
                fmt.Printf("Proxy iniciado na porta %d (PID: %d)\n", port, getPID(port))
            }
            waitEnter()

        case "2":
            clearScreen()
            fmt.Print("Porta para fechar: ")
            if !scanner.Scan() {
                return
            }
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if err := StopProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Porta %d fechada com sucesso.\n", port)
            }
            waitEnter()

        case "3":
            clearScreen()
            fmt.Print("Porta para ver logs: ")
            if !scanner.Scan() {
                return
            }
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            ViewLogs(port)
            waitEnter()

        case "4":
            clearScreen()
            instances = loadState()
            if len(instances) == 0 {
                fmt.Println("Nenhuma porta ativa no momento.")
            } else {
                fmt.Println("PORTAS ATIVAS:")
                fmt.Println("┌───────┬───────┐")
                fmt.Printf("│ %-5s │ %-5s │\n", "PORTA", "PID")
                fmt.Println("├───────┼───────┤")
                for p, inst := range instances {
                    fmt.Printf("│ %-5d │ %-5d │\n", p, inst.PID)
                }
                fmt.Println("└───────┴───────┘")
            }
            waitEnter()

        case "5":
            clearScreen()
            fmt.Println("Saindo do Proxy Go 2.0...")
            fmt.Println("Portas ativas continuam rodando em background!")
            time.Sleep(1)
            return

        default:
            clearScreen()
            fmt.Println("Opção inválida! Tente novamente.")
            waitEnter()
        }
    }
}

// === INICIAR PROXY EM BACKGROUND ===
func StartBackgroundProxy(port int) error {
    cmd := exec.Command("nohup", os.Args[0], "-port="+strconv.Itoa(port), ">/dev/null", "2>&1", "&")
    cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
    if err := cmd.Start(); err != nil {
        return err
    }

    inst := &ProxyInstance{Port: port, PID: cmd.Process.Pid}
    instances := loadState()
    instances[port] = inst
    saveState(instances)
    return nil
}

// === PARAR PROXY ===
func StopProxy(port int) error {
    instances := loadState()
    inst, exists := instances[port]
    if !exists {
        return fmt.Errorf("porta %d não está ativa", port)
    }

    proc, err := os.FindProcess(inst.PID)
    if err != nil {
        delete(instances, port)
        saveState(instances)
        return nil
    }
    proc.Signal(syscall.SIGKILL)
    delete(instances, port)
    saveState(instances)
    return nil
}

// === VER LOGS ===
func ViewLogs(port int) {
    logFile := fmt.Sprintf("/var/log/proxy-go2.0-%d.log", port)
    data, err := os.ReadFile(logFile)
    if err != nil {
        fmt.Printf("Nenhum log encontrado para a porta %d.\n", port)
        return
    }

    lines := strings.Split(string(data), "\n")
    start := len(lines) - 20
    if start < 0 {
        start = 0
    }

    fmt.Printf("Últimos 20 logs da porta %d:\n\n", port)
    for i := start; i < len(lines); i++ {
        if strings.TrimSpace(lines[i]) != "" {
            fmt.Printf("  %s\n", lines[i])
        }
    }
}

// === PEGAR PID (opcional) ===
func getPID(port int) int {
    instances := loadState()
    if inst, ok := instances[port]; ok {
        return inst.PID
    }
    return 0
}
