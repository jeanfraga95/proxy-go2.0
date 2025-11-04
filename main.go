// main.go
package main

import (
    "flag"
    "fmt"
    "log"
)

func main() {
    portFlag := flag.Int("port", 0, "Iniciar proxy diretamente na porta (sem menu)")
    flag.Parse()

    if *portFlag > 0 {
        log.Printf("Iniciando proxy na porta %d (modo background)...\n", *portFlag)
        if err := StartServer(*portFlag); err != nil {
            log.Fatal("Erro ao iniciar servidor: ", err)
        }
        select {} // Mantém vivo
    } else {
        fmt.Println("=== Proxy Go 2.0 ===")
        RunMenu()
    }
}
