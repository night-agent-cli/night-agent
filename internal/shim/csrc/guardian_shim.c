#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
    #include <windows.h>
    #include <io.h>
    #include <process.h>
    #include <direct.h>
    #define getcwd _getcwd
    #define access _access
    #define X_OK 0
    // Winsock per i socket
    #include <winsock2.h>
    #include <ws2tcpip.h>
    #include <afunix.h> // Per AF_UNIX su Windows moderno
    #pragma comment(lib, "ws2_32.lib")
    
    // Sostituto per basename
    static char *win_basename(char *path) {
        char *p = strrchr(path, '\\');
        if (!p) p = strrchr(path, '/');
        return p ? p + 1 : path;
    }
    #define basename win_basename
    #define PATH_SEP ";"
#else
    #include <unistd.h>
    #include <sys/socket.h>
    #include <sys/un.h>
    #include <libgen.h>
    #define PATH_SEP ":"
#endif

#define SOCKET_ENV   "GUARDIAN_SOCKET"
#define SHIM_DIR_ENV "GUARDIAN_SHIM_DIR"
#define MAX_CMD      8192
#define MAX_RESP     4096

#define GUARDIAN_ALLOW   1
#define GUARDIAN_BLOCK   0
#define GUARDIAN_SANDBOX (-1)

// --- Helper Functions (parse_reason, parse_exit_code rimangono identiche) ---
static char *parse_reason(const char *resp, char *buf, size_t buf_size) {
    buf[0] = '\0';
    const char *key = "\"reason\":\"";
    const char *p = strstr(resp, key);
    if (!p) return buf;
    p += strlen(key);
    size_t i = 0;
    while (*p && *p != '"' && i < buf_size - 1) {
        if (*p == '\\' && *(p+1) == '"') { buf[i++] = '"'; p += 2; }
        else if (*p == '\\' && *(p+1) == 'n') { buf[i++] = ' '; p += 2; }
        else { buf[i++] = *p++; }
    }
    buf[i] = '\0';
    return buf;
}

static int parse_exit_code(const char *resp) {
    const char *key = "\"exit_code\":";
    const char *p = strstr(resp, key);
    if (!p) return 0;
    p += strlen(key);
    while (*p == ' ') p++;
    return atoi(p);
}

// --- Guardian Check (Versione Windows/Unix) ---
static int guardian_check(const char *socket_path, const char *command, int *out_exit_code,
                           char *resp_buf, size_t resp_buf_size) {
    *out_exit_code = 0;

#ifdef _WIN32
    WSADATA wsaData;
    WSAStartup(MAKEWORD(2, 2), &wsaData);
#endif

    int fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (fd < 0) return GUARDIAN_BLOCK;

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, socket_path, sizeof(addr.sun_path) - 1);

    if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
#ifdef _WIN32
        closesocket(fd);
        WSACleanup();
#else
        close(fd);
#endif
        return GUARDIAN_BLOCK;
    }

    // Costruzione richiesta JSON (semplificata)
    char workdir[4096];
    getcwd(workdir, sizeof(workdir));
    const char *agent_name = getenv("NIGHTAGENT_AGENT") ? getenv("NIGHTAGENT_AGENT") : "shim";

    char req[MAX_CMD + 1024];
    snprintf(req, sizeof(req), "{\"command\":\"%s\",\"work_dir\":\"%s\",\"agent_name\":\"%s\"}\n",
             command, workdir, agent_name);

    send(fd, req, strlen(req), 0);

    char resp[MAX_RESP];
    int n = recv(fd, resp, sizeof(resp) - 1, 0);
    if (n > 0) resp[n] = '\0';

#ifdef _WIN32
    closesocket(fd);
    WSACleanup();
#else
    close(fd);
#endif

    if (n <= 0) return GUARDIAN_BLOCK;
    if (resp_buf) strncpy(resp_buf, resp, resp_buf_size);

    if (strstr(resp, "\"allow\"")) return GUARDIAN_ALLOW;
    return GUARDIAN_BLOCK;
}

static char *find_real_binary(const char *cmdname, const char *shim_dir) {
    const char *path_env = getenv("PATH");
    if (!path_env) return NULL;

    char *path_copy = strdup(path_env);
    static char result[4096];
    char *dir = strtok(path_copy, PATH_SEP);
    while (dir) {
        if (shim_dir == NULL || strcmp(dir, shim_dir) != 0) {
            // Prova con .exe e senza
            snprintf(result, sizeof(result), "%s/%s", dir, cmdname);
            if (access(result, X_OK) == 0) { free(path_copy); return result; }
            snprintf(result, sizeof(result), "%s/%s.exe", dir, cmdname);
            if (access(result, X_OK) == 0) { free(path_copy); return result; }
        }
        dir = strtok(NULL, PATH_SEP);
    }
    free(path_copy);
    return NULL;
}

int main(int argc, char *argv[]) {
    char argv0_copy[4096];
    strncpy(argv0_copy, argv[0], sizeof(argv0_copy) - 1);
    char *cmdname = basename(argv0_copy);

    char full_cmd[MAX_CMD] = {0};
    for (int i = 0; i < argc; i++) {
        strncat(full_cmd, argv[i], sizeof(full_cmd) - strlen(full_cmd) - 1);
        if (i < argc - 1) strncat(full_cmd, " ", sizeof(full_cmd) - strlen(full_cmd) - 1);
    }

    const char *socket_path = getenv(SOCKET_ENV);
    const char *shim_dir    = getenv(SHIM_DIR_ENV);

    if (socket_path != NULL) {
        int exit_code = 0;
        char raw_resp[MAX_RESP] = {0};
        int result = guardian_check(socket_path, full_cmd, &exit_code, raw_resp, sizeof(raw_resp));

        if (result == GUARDIAN_BLOCK) {
            char reason[512];
            parse_reason(raw_resp, reason, sizeof(reason));
            fprintf(stderr, "[guardian] bloccato: %s\n", reason);
            return 1;
        }
    }

    char *real_binary = find_real_binary(cmdname, shim_dir);
    if (!real_binary) {
        fprintf(stderr, "guardian-shim: %s: command not found\n", cmdname);
        return 127;
    }

#ifdef _WIN32
    // Su Windows execvp non sostituisce davvero il processo, usiamo spawnvp
    intptr_t ret = _spawnvp(_P_WAIT, real_binary, (const char* const*)argv);
    return (int)ret;
#else
    execvp(real_binary, argv);
    return 1;
#endif
}