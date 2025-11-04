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

var instances = make(map[int]*ProxyInstance)
var mu sync.Mutex

func Menu() {
    scanner := bufio.NewScanner(os.Stdin)
    for {
        fmt.Println("\n--- Menu Proxy ---")
        fmt.Println("1. Abrir porta")
        fmt.Println("2. Fechar porta")
        fmt.Println("3. Ver logs da porta")
        fmt.Println("4. Listar portas ativas")
        fmt.Println("5. Sair")
        fmt.Print("Escolha: ")
        scanner.Scan()
        choice := strings.TrimSpace(scanner.Text())

        switch choice {
        case "1":
            fmt.Print("Porta: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if err := StartProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Proxy iniciado na porta %d (PID: %d)\n", port, instances[port].PID)
            }
        case "2":
            fmt.Print("Porta: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            if err := StopProxy(port); err != nil {
                fmt.Printf("Erro: %v\n", err)
            } else {
                fmt.Printf("Proxy fechado na porta %d\n", port)
            }
        case "3":
            fmt.Print("Porta: ")
            scanner.Scan()
            port, _ := strconv.Atoi(strings.TrimSpace(scanner.Text()))
            ViewLogs(port)
        case "4":
            ListPorts()
        case "5":
            fmt.Println("Saindo...")
            for _, inst := range instances {
                StopProxy(inst.Port)
            }
            return
        default:
            fmt.Println("Opção inválida!")
        }
    }
}

func StartProxy(port int) error {
    mu.Lock()
    defer mu.Unlock()
    if _, exists := instances[port]; exists {
        return fmt.Errorf("porta %d já em uso", port)
    }

    // Inicia como processo background (nohup para detach)
    cmd := exec.Command("nohup", os.Args[0], "-port="+strconv.Itoa(port))
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setsid: true, // Novo session para background
    }
    if err := cmd.Start(); err != nil {
        return err
    }

    inst := &ProxyInstance{
        Port:    port,
        Cmd:     cmd,
        PID:     cmd.Process.Pid,
        Running: true,
    }
    instances[port] = inst

    // Log inicial
    logFile := fmt.Sprintf("/var/log/proxy-%d.log", port)
    go tailLogs(logFile, inst) // Goroutine para capturar logs em tempo real

    return nil
}

func StopProxy(port int) error {
    mu.Lock()
    defer mu.Unlock()
    inst, exists := instances[port]
    if !exists || !inst.Running {
        return fmt.Errorf("porta %d não está rodando", port)
    }

    if err := inst.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
        return err
    }
    inst.Running = false
    delete(instances, port)
    return nil
}

func ViewLogs(port int) {
    mu.Lock()
    inst, exists := instances[port]
    mu.Unlock()
    if !exists {
        fmt.Printf("Porta %d não encontrada\n", port)
        return
    }
    fmt.Printf("Logs da porta %d (últimos 10):\n", port)
    inst.Mutex.Lock()
    for i, log := range inst.Logs[len(inst.Logs)-10:] {
        fmt.Printf("%d: %s\n", i, log)
    }
    inst.Mutex.Unlock()
}

func ListPorts() {
    fmt.Println("Portas ativas:")
    for port, inst := range instances {
        status := "RUNNING"
        if !inst.Running {
            status = "STOPPED"
        }
        fmt.Printf("Porta %d (PID: %d) - %s\n", port, inst.PID, status)
    }
}

func tailLogs(logFile string, inst *ProxyInstance) {
    // Simula captura de logs (em prod, use tail -f ou journalctl)
    for {
        time.Sleep(5 * time.Second)
        // Adicione logs simulados ou leia arquivo real
        inst.Mutex.Lock()
        inst.Logs = append(inst.Logs, time.Now().Format("2006-01-02 15:04:05")+" - Conexão recebida")
        if len(inst.Logs) > 100 { // Limite
            inst.Logs = inst.Logs[1:]
        }
        inst.Mutex.Unlock()
    }
}
