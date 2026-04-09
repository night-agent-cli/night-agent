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
 * Effettua la verifica con il daemon Guardian.
 * Ritorna 1 se il comando è consentito, 0 se bloccato o in caso di errore.
 */
static int guardian_check(const char *socket_path, const char *command)
{
    int fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (fd < 0) {
        return 0; /* safe failure: block */
    }

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, socket_path, sizeof(addr.sun_path) - 1);

    if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        close(fd);
        return 0; /* safe failure: block */
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

    /* invia richiesta JSON */
    char req[MAX_CMD + 128];
    snprintf(req, sizeof(req),
        "{\"command\":\"%s\",\"work_dir\":\"\",\"agent_name\":\"shim\"}\n",
        escaped);

    if (write(fd, req, strlen(req)) < 0) {
        close(fd);
        return 0;
    }
    shutdown(fd, SHUT_WR);

    /* leggi risposta */
    char resp[MAX_RESP];
    ssize_t n = read(fd, resp, sizeof(resp) - 1);
    close(fd);

    if (n <= 0) {
        return 0; /* safe failure */
    }
    resp[n] = '\0';

    /* il daemon risponde con JSON contenente "decision":"allow" o "block" */
    return strstr(resp, "\"allow\"") != NULL;
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
        int allowed = guardian_check(socket_path, full_cmd);
        if (!allowed) {
            fprintf(stderr, "\033[31m[guardian] bloccato: %s\033[0m\n", full_cmd);
            return 1;
        }
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
