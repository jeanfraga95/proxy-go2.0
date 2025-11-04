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

func clearScreen() { exec.Command("clear").Run() }

func loadState() map[int]*ProxyInstance {
    mu.Lock()
    defer mu.Unlock()
    data, _ := os.ReadFile(stateFile)
    var state struct{ Instances map[int]*ProxyInstance `json:"instances"` }
    if json.Unmarshal(data, &state) == nil && state.Instances != nil {
        return state.Instances
    }
    return make(map[int]*ProxyInstance)
}

func saveState(instances map[int]*ProxyInstance) {
    mu.Lock()
    defer mu.Unlock()
    data, _ := json.MarshalIndent(map[string]any{"instances": instances}, "", "  ")
    os.WriteFile(stateFile, data, 0644)
}

func RunMenu() {
    instances := loadState()
    scanner := bufio.NewScanner(os.Stdin)

    for {
        clearScreen()
        fmt.Println("╔════════════════════════════════════╗")
        fmt.Println("║      PROXY GO 2.0 - TUNNEL         ║")
        fmt.Println("╚════════════════════════════════════╝\n")
        fmt.Println("1. Abrir porta")
        fmt.Println("2. Fechar porta")
        fmt.Println("3. Ver logs")
        fmt.Println("4. Listar portas ativas")
        fmt.Println("5. Sair")
        fmt.Print("\nEscolha: ")

        if !scanner.Scan() { break }
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
            if _, exists := instances[port]; exists {
                fmt.Println("Porta já em uso!")
            } else if err := StartBackgroundProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Proxy iniciado na porta %d\n", port)
                instances = loadState() // recarrega
            }
            fmt.Print("\nENTER...")
            scanner.Scan()

        case "4":
            clearScreen()
            instances = loadState()
            if len(instances) == 0 {
                fmt.Println("Nenhuma porta ativa.")
            } else {
                fmt.Println("PORTAS ATIVAS:")
                for p, i := range instances {
                    fmt.Printf("  → %d (PID: %d)\n", p, i.PID)
                }
            }
            fmt.Print("\nENTER...")
            scanner.Scan()
        }
    }
}

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
