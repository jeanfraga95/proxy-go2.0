// main.go
package main

import (
    "flag"
    "fmt"
    "log"
)

func main() {
    port := flag.Int("port", 0, "Iniciar proxy na porta")
    flag.Parse()

    if *port > 0 {
        if err := StartServer(*port); err != nil {
            log.Fatal(err)
        }
        select {}
    } else {
        RunMenu()
    }
}
