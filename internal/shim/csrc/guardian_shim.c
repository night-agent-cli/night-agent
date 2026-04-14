/*
 * guardian_shim.c
 *
 * Shim binario per PATH interception agnostica dagli agenti AI.
 * Installato come symlink nella shim directory:
 *   ~/.guardian/shims/sudo  → guardian-shim
 *   ~/.guardian/shims/rm    → guardian-shim
 *   ...
 *
 * Comportamento:
 *   1. Legge argv[0] (basename) per sapere quale comando sta sostituendo
 *   2. Costruisce il comando completo con tutti gli argomenti
 *   3. Contatta il daemon Guardian via GUARDIAN_SOCKET (Unix socket)
 *   4. Se allow → trova il binario reale nel PATH (saltando la shim dir) ed execvp
 *   5. Se block → stampa messaggio e ritorna 1
 *   6. Safe failure (daemon non raggiungibile) → block
 *
 * Se GUARDIAN_SOCKET non è impostato (fuori da guardian run), passa sempre.
 *
 * Compilazione: clang -o guardian-shim guardian_shim.c
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <libgen.h>

#define SOCKET_ENV   "GUARDIAN_SOCKET"
#define SHIM_DIR_ENV "GUARDIAN_SHIM_DIR"
#define MAX_CMD      8192
#define MAX_RESP     4096

/*
 * Risultato della verifica con il daemon Guardian.
 *   GUARDIAN_ALLOW   = 1  → esegui il comando normalmente
 *   GUARDIAN_BLOCK   = 0  → blocca, stampa errore
 *   GUARDIAN_SANDBOX = -1 → il daemon ha già eseguito in Docker, esci con exit_code
 */
#define GUARDIAN_ALLOW   1
#define GUARDIAN_BLOCK   0
#define GUARDIAN_SANDBOX (-1)

/*
 * Estrae il valore stringa di "reason" dal JSON della risposta.
 * Scrive al massimo buf_size-1 caratteri in buf. Ritorna buf.
 */
static char *parse_reason(const char *resp, char *buf, size_t buf_size)
{
    buf[0] = '\0';
    const char *key = "\"reason\":\"";
    const char *p = strstr(resp, key);
    if (!p) return buf;
    p += strlen(key);

    size_t i = 0;
    while (*p && *p != '"' && i < buf_size - 1) {
        if (*p == '\\' && *(p+1) == '"') {
            buf[i++] = '"';
            p += 2;
        } else if (*p == '\\' && *(p+1) == 'n') {
            buf[i++] = ' ';
            p += 2;
        } else {
            buf[i++] = *p++;
        }
    }
    buf[i] = '\0';
    return buf;
}

/*
 * Estrae il valore intero di "exit_code" dal JSON della risposta.
 * Ritorna 0 se non trovato o non parsabile.
 */
static int parse_exit_code(const char *resp)
{
    const char *key = "\"exit_code\":";
    const char *p = strstr(resp, key);
    if (!p) return 0;
    p += strlen(key);
    /* salta spazi */
    while (*p == ' ') p++;
    return atoi(p);
}

/*
 * Stampa il campo "output" dalla risposta JSON (stdout del container).
 * Il valore è racchiuso tra virgolette ed usa \n come escape.
 */
static void print_sandbox_output(const char *resp)
{
    const char *key = "\"output\":\"";
    const char *p = strstr(resp, key);
    if (!p) return;
    p += strlen(key);

    char buf[MAX_RESP];
    size_t i = 0;
    while (*p && *p != '"' && i < sizeof(buf) - 2) {
        if (*p == '\\' && *(p+1) == 'n') {
            buf[i++] = '\n';
            p += 2;
        } else if (*p == '\\' && *(p+1) == '"') {
            buf[i++] = '"';
            p += 2;
        } else {
            buf[i++] = *p++;
        }
    }
    buf[i] = '\0';
    if (i > 0) {
        fputs(buf, stdout);
    }
}

/*
 * Effettua la verifica con il daemon Guardian.
 * Ritorna GUARDIAN_ALLOW, GUARDIAN_BLOCK o GUARDIAN_SANDBOX.
 * In caso di GUARDIAN_SANDBOX, *out_exit_code contiene l'exit code del container.
 * resp_buf (se non NULL) riceve la risposta JSON grezza per ulteriore parsing.
 */
static int guardian_check(const char *socket_path, const char *command, int *out_exit_code,
                           char *resp_buf, size_t resp_buf_size)
{
    *out_exit_code = 0;

    int fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (fd < 0) {
        return GUARDIAN_BLOCK; /* safe failure: block */
    }

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, socket_path, sizeof(addr.sun_path) - 1);

    if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        close(fd);
        return GUARDIAN_BLOCK; /* safe failure: block */
    }

    /* escape base: sostituisce " con ' nel comando per non rompere il JSON */
    char escaped[MAX_CMD];
    size_t j = 0;
    for (size_t i = 0; command[i] && j < sizeof(escaped) - 2; i++) {
        if (command[i] == '"') {
            escaped[j++] = '\'';
        } else if (command[i] == '\\') {
            escaped[j++] = '\\';
            escaped[j++] = '\\';
        } else {
            escaped[j++] = command[i];
        }
    }
    escaped[j] = '\0';

    /* ottieni la directory di lavoro corrente */
    char workdir[4096];
    if (getcwd(workdir, sizeof(workdir)) == NULL) {
        workdir[0] = '\0';
    }

    /* agent_name: usa NIGHTAGENT_AGENT se settata, altrimenti "shim" */
    const char *agent_name = getenv("NIGHTAGENT_AGENT");
    if (agent_name == NULL || agent_name[0] == '\0') {
        agent_name = "shim";
    }

    /* invia richiesta JSON */
    char req[MAX_CMD + 4096 + 64];
    snprintf(req, sizeof(req),
        "{\"command\":\"%s\",\"work_dir\":\"%s\",\"agent_name\":\"%s\"}\n",
        escaped, workdir, agent_name);

    if (write(fd, req, strlen(req)) < 0) {
        close(fd);
        return GUARDIAN_BLOCK;
    }
    shutdown(fd, SHUT_WR);

    /* leggi risposta (potenzialmente lunga per output sandbox) */
    char resp[MAX_RESP];
    ssize_t total = 0;
    ssize_t n;
    while (total < (ssize_t)(sizeof(resp) - 1)) {
        n = read(fd, resp + total, sizeof(resp) - 1 - (size_t)total);
        if (n <= 0) break;
        total += n;
    }
    close(fd);

    if (total <= 0) {
        return GUARDIAN_BLOCK; /* safe failure */
    }
    resp[total] = '\0';

    /* copia risposta nel buffer esterno se fornito */
    if (resp_buf && resp_buf_size > 0) {
        strncpy(resp_buf, resp, resp_buf_size - 1);
        resp_buf[resp_buf_size - 1] = '\0';
    }

    /* determina la decisione dalla risposta JSON */
    if (strstr(resp, "\"sandbox\"")) {
        *out_exit_code = parse_exit_code(resp);
        print_sandbox_output(resp);
        return GUARDIAN_SANDBOX;
    }
    if (strstr(resp, "\"allow\"")) {
        return GUARDIAN_ALLOW;
    }
    return GUARDIAN_BLOCK;
}

/*
 * Trova il binario reale nel PATH saltando la shim directory.
 * Ritorna il path del binario o NULL se non trovato.
 * Usa un buffer statico — non rientrante, ma sufficiente per questo uso.
 */
static char *find_real_binary(const char *cmdname, const char *shim_dir)
{
    const char *path_env = getenv("PATH");
    if (!path_env) {
        return NULL;
    }

    char *path_copy = strdup(path_env);
    if (!path_copy) {
        return NULL;
    }

    static char result[4096];
    char *dir = strtok(path_copy, ":");
    while (dir) {
        /* salta la shim directory */
        if (shim_dir == NULL || strcmp(dir, shim_dir) != 0) {
            snprintf(result, sizeof(result), "%s/%s", dir, cmdname);
            if (access(result, X_OK) == 0) {
                free(path_copy);
                return result;
            }
        }
        dir = strtok(NULL, ":");
    }

    free(path_copy);
    return NULL;
}

int main(int argc, char *argv[])
{
    /*
     * argv[0] è il path completo del symlink (es. /home/user/.guardian/shims/sudo).
     * basename() restituisce solo il nome del comando (es. "sudo").
     */
    char argv0_copy[4096];
    strncpy(argv0_copy, argv[0], sizeof(argv0_copy) - 1);
    argv0_copy[sizeof(argv0_copy) - 1] = '\0';
    char *cmdname = basename(argv0_copy);

    /* costruisci il comando completo con tutti gli argomenti */
    char full_cmd[MAX_CMD];
    size_t pos = 0;
    pos += (size_t)snprintf(full_cmd + pos, sizeof(full_cmd) - pos, "%s", cmdname);
    for (int i = 1; i < argc && pos < sizeof(full_cmd) - 2; i++) {
        pos += (size_t)snprintf(full_cmd + pos, sizeof(full_cmd) - pos, " %s", argv[i]);
    }

    const char *socket_path = getenv(SOCKET_ENV);
    const char *shim_dir    = getenv(SHIM_DIR_ENV);

    if (socket_path != NULL) {
        int sandbox_exit_code = 0;
        char raw_resp[MAX_RESP];
        raw_resp[0] = '\0';
        int result = guardian_check(socket_path, full_cmd, &sandbox_exit_code,
                                    raw_resp, sizeof(raw_resp));

        if (result == GUARDIAN_BLOCK) {
            char reason[512];
            parse_reason(raw_resp, reason, sizeof(reason));
            if (reason[0]) {
                fprintf(stderr, "\033[31m[guardian] bloccato: %s — %s\033[0m\n", full_cmd, reason);
            } else {
                fprintf(stderr, "\033[31m[guardian] bloccato: %s\033[0m\n", full_cmd);
            }
            return 1;
        }
        if (result == GUARDIAN_SANDBOX) {
            /* stampa header sandbox su stderr prima dell'output del container */
            char reason[256];
            parse_reason(raw_resp, reason, sizeof(reason));
            if (reason[0]) {
                fprintf(stderr, "\033[33m[⬡ sandbox]\033[0m %s — %s\n", full_cmd, reason);
            } else {
                fprintf(stderr, "\033[33m[⬡ sandbox]\033[0m %s\n", full_cmd);
            }
            /* il daemon ha già eseguito il comando in Docker — propaga exit code */
            return sandbox_exit_code;
        }
        /* GUARDIAN_ALLOW: continua con execvp sotto */
    }
    /*
     * Se GUARDIAN_SOCKET non è impostato siamo fuori da 'guardian run':
     * passa sempre (comportamento trasparente per uso normale del sistema).
     */

    /* trova il binario reale nel PATH, saltando la shim dir */
    char *real_binary = find_real_binary(cmdname, shim_dir);
    if (!real_binary) {
        fprintf(stderr, "guardian-shim: %s: command not found\n", cmdname);
        return 127;
    }

    /* sostituisci il processo con il binario reale */
    execvp(real_binary, argv);
    perror("guardian-shim: execvp");
    return 1;
}
