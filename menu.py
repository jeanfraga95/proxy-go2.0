#!/usr/bin/env python3
import os
import json
import subprocess
import signal
import sys
import time

INSTALL_DIR = "/opt/proxy-go2.0"
STATE_FILE = f"{INSTALL_DIR}/ports.json"
PROXY_BIN = f"{INSTALL_DIR}/proxy"

def clear():
    os.system('clear')

def load_state():
    if not os.path.exists(STATE_FILE):
        return {}
    with open(STATE_FILE, 'r') as f:
        return json.load(f).get("instances", {})

def save_state(state):
    with open(STATE_FILE, 'w') as f:
        json.dump({"instances": state}, f, indent=2)

def start_proxy(port):
    if str(port) in load_state():
        return "Já em uso"
    cmd = ["nohup", PROXY_BIN, str(port), ">/dev/null", "2>&1", "&"]
    subprocess.Popen(cmd)
    time.sleep(1)
    instances = load_state()
    instances[str(port)] = {"port": port, "pid": get_pid(port)}
    save_state(instances)
    return f"Porta {port} aberta"

def stop_proxy(port):
    instances = load_state()
    port_str = str(port)
    if port_str not in instances:
        return "Não encontrada"
    pid = instances[port_str]["pid"]
    try:
        os.kill(pid, signal.SIGKILL)
    except:
        pass
    del instances[port_str]
    save_state(instances)
    return f"Porta {port} fechada"

def get_pid(port):
    result = subprocess.run(["ps", "aux"], capture_output=True, text=True)
    for line in result.stdout.splitlines():
        if f"proxy {port}" in line:
            return int(line.split()[1])
    return 0

def view_logs(port):
    log_file = f"/var/log/proxy-go2.0-{port}.log"
    if not os.path.exists(log_file):
        return "Sem logs"
    return subprocess.getoutput(f"tail -20 {log_file}")

def main():
    while True:
        clear()
        print("╔════════════════════════════════════╗")
        print("║      PROXY GO 2.0 - TUNNEL         ║")
        print("╚════════════════════════════════════╝\n")
        print("1. Abrir porta")
        print("2. Fechar porta")
        print("3. Ver logs")
        print("4. Listar portas ativas")
        print("5. Sair\n")
        choice = input("Escolha: ").strip()

        if choice == "1":
            clear()
            port = input("Porta: ").strip()
            if not port.isdigit() or not (1 <= int(port) <= 65535):
                print("Porta inválida!")
            else:
                print(start_proxy(int(port)))
            input("\nENTER para continuar...")

        elif choice == "2":
            clear()
            port = input("Porta para fechar: ").strip()
            if port.isdigit():
                print(stop_proxy(int(port)))
            input("\nENTER para continuar...")

        elif choice == "3":
            clear()
            port = input("Porta para logs: ").strip()
            if port.isdigit():
                print(view_logs(int(port)))
            input("\nENTER para continuar...")

        elif choice == "4":
            clear()
            instances = load_state()
            if not instances:
                print("Nenhuma porta ativa.")
            else:
                print("PORTAS ATIVAS:")
                for p, i in instances.items():
                    print(f"  → {p} (PID: {i['pid']})")
            input("\nENTER para continuar...")

        elif choice == "5":
            clear()
            print("Saindo... Portas continuam ativas!")
            time.sleep(1)
            break

if __name__ == "__main__":
    main()
