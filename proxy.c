// proxy.c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <arpa/inet.h>
#include <sys/socket.h>
#include <pthread.h>
#include <openssl/ssl.h>
#include <openssl/err.h>
#include <sys/wait.h>

#define MAX_PORT 65535
#define LOG_DIR "/var/log"
#define CERT_DIR "/tmp"

typedef struct {
    int port;
} proxy_args_t;

FILE *log_file;
char log_path[256];

void log_msg(const char *msg) {
    time_t now = time(NULL);
    char *t = ctime(&now);
    t[strlen(t)-1] = '\0';
    fprintf(log_file, "[%s] %s\n", t, msg);
    fflush(log_file);
}

void generate_cert(int port) {
    char cert[128], key[128], cmd[512];
    snprintf(cert, sizeof(cert), "%s/cert-%d.pem", CERT_DIR, port);
    snprintf(key, sizeof(key), "%s/key-%d.pem", CERT_DIR, port);
    snprintf(cmd, sizeof(cmd),
             "openssl req -new -newkey rsa:2048 -days 365 -nodes -x509 "
             "-keyout %s -out %s -subj '/CN=localhost' 2>/dev/null",
             key, cert);
    system(cmd);
}

SSL_CTX *init_ssl(int port) {
    char cert[128], key[128];
    snprintf(cert, sizeof(cert), "%s/cert-%d.pem", CERT_DIR, port);
    snprintf(key, sizeof(key), "%s/key-%d.pem", CERT_DIR, port);

    SSL_library_init();
    SSL_CTX *ctx = SSL_CTX_new(TLS_server_method());
    if (!ctx) return NULL;

    if (SSL_CTX_use_certificate_file(ctx, cert, SSL_FILETYPE_PEM) <= 0 ||
        SSL_CTX_use_PrivateKey_file(ctx, key, SSL_FILETYPE_PEM) <= 0) {
        SSL_CTX_free(ctx);
        return NULL;
    }
    return ctx;
}

void send_response(int fd, const char *resp) {
    send(fd, resp, strlen(resp), 0);
    log_msg((char*)resp);
}

void authenticate_ssh(int client_fd) {
    int pipefd[2];
    if (pipe(pipefd) == -1) return;

    pid_t pid = fork();
    if (pid == 0) {
        close(pipefd[0]);
        dup2(client_fd, 0);
        dup2(client_fd, 1);
        dup2(pipefd[1], 2);
        close(pipefd[1]);
        execlp("ssh", "ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no",
               "localhost", "true", (char*)NULL);
        exit(1);
    } else if (pid > 0) {
        close(pipefd[1]);
        char buf[256];
        int n;
        while ((n = read(pipefd[0], buf, sizeof(buf))) > 0) {
            // Ignore output
        }
        close(pipefd[0]);
        waitpid(pid, NULL, 0);
    }
}

void *handle_client(void *arg) {
    int client_fd = *(int*)arg;
    free(arg);

    char buf[1];
    int n = recv(client_fd, buf, 1, MSG_PEEK);
    if (n <= 0) { close(client_fd); return NULL; }

    if (buf[0] == 0x05) {
        // SOCKS5 → HTTP Injector
        send_response(client_fd, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n");
        log_msg("SOCKS5 conectado → Autenticando via SSH...");
        recv(client_fd, buf, 1, 0); // consume
        send(client_fd, "\x05\x00", 2, 0);
        authenticate_ssh(client_fd);
        log_msg("Túnel SOCKS5 ativo");
        char buffer[4096];
        while ((n = recv(client_fd, buffer, sizeof(buffer), 0)) > 0)
            send(client_fd, buffer, n, 0);
    }
    else if (buf[0] == 0x16) {
        // WSS
        SSL_CTX *ctx = init_ssl(((proxy_args_t*)arg)->port);
        if (!ctx) { close(client_fd); return NULL; }
        SSL *ssl = SSL_new(ctx);
        SSL_set_fd(ssl, client_fd);
        if (SSL_accept(ssl) <= 0) {
            SSL_free(ssl); SSL_CTX_free(ctx); close(client_fd); return NULL;
        }
        send_response(client_fd, "HTTP/1.1 101 PROXY-GO2.0\r\n\r\n");
        log_msg("WSS conectado → Autenticando via SSH...");
        authenticate_ssh(client_fd);
        log_msg("Túnel WSS ativo");
        char buffer[4096];
        while ((n = SSL_read(ssl, buffer, sizeof(buffer))) > 0)
            SSL_write(ssl, buffer, n);
        SSL_free(ssl); SSL_CTX_free(ctx);
    }
    else {
        send_response(client_fd, "HTTP/1.1 200 PROXY-GO2.0\r\n\r\n");
        log_msg("HTTP CONNECT → Autenticando...");
        authenticate_ssh(client_fd);
        char buffer[4096];
        while ((n = recv(client_fd, buffer, sizeof(buffer), 0)) > 0)
            send(client_fd, buffer, n, 0);
    }
    close(client_fd);
    return NULL;
}

void start_proxy(int port) {
    snprintf(log_path, sizeof(log_path), "%s/proxy-go2.0-%d.log", LOG_DIR, port);
    log_file = fopen(log_path, "a");
    if (!log_file) exit(1);

    char cert[128];
    snprintf(cert, sizeof(cert), "%s/cert-%d.pem", CERT_DIR, port);
    if (access(cert, F_OK) == -1) generate_cert(port);

    int server_fd = socket(AF_INET, SOCK_STREAM, 0);
    int opt = 1;
    setsockopt(server_fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

    struct sockaddr_in addr = {0};
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = INADDR_ANY;
    addr.sin_port = htons(port);

    if (bind(server_fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        log_msg("Erro ao bindar porta");
        exit(1);
    }
    listen(server_fd, 10);
    log_msg("PROXY-GO2.0 RODANDO NA PORTA %d");

    while (1) {
        int *client_fd = malloc(sizeof(int));
        *client_fd = accept(server_fd, NULL, NULL);
        if (*client_fd < 0) { free(client_fd); continue; }

        proxy_args_t *args = malloc(sizeof(proxy_args_t));
        args->port = port;
        pthread_t t;
        pthread_create(&t, NULL, handle_client, client_fd);
        pthread_detach(t);
    }
    fclose(log_file);
}
