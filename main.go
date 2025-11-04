// main.go
package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    portFlag := flag.Int("port", 0, "Porta para iniciar o proxy diretamente (sem menu)")
    flag.Parse()

    if *portFlag > 0 {
        // Modo direto: inicia proxy na porta sem menu
        if err := StartProxy(*portFlag); err != nil {
            log.Fatal(err)
        }
        select {} // Mantém rodando
    } else {
        // Modo menu interativo
        fmt.Println("=== Proxy Go 2.0 ===")
        Menu()
    }
}
