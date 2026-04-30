#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <stddef.h>

#ifdef _WIN32
    #include <windows.h>
    #include <process.h>
    #include <winsock2.h>
    #include <ws2tcpip.h>
    
    // Rimuoviamo la nostra definizione di pid_t perché esiste già in sys/types.h
    // Includiamo sys/types.h esplicitamente per sicurezza
    #include <sys/types.h>

    typedef void* posix_spawn_file_actions_t;
    typedef void* posix_spawnattr_t;
    
    #ifndef EPERM
        #define EPERM 1
    #endif
    #ifndef ENOSYS
        #define ENOSYS 40
    #endif

    #define strlcpy(dst, src, sz) snprintf(dst, sz, "%s", src)
    #define strlcat(dst, src, sz) strncat(dst, src, sz - strlen(dst) - 1)
#else
    #define _GNU_SOURCE
    #include <dlfcn.h>
    #include <spawn.h>
    #include <unistd.h>
    #include <sys/socket.h>
    #include <sys/un.h>
    #include <sys/types.h>
#endif

/* ---------- debug ---------------------------------------------------- */

static int debug_enabled(void)
{
    return getenv("GUARDIAN_DEBUG") != NULL;
}

#define DBG(fmt, ...) do { \
    if (debug_enabled()) \
        fprintf(stderr, "[guardian] " fmt "\n", ##__VA_ARGS__); \
} while (0)

#ifndef _WIN32
/* ---------- DYLD_INTERPOSE macro (Solo macOS) ------------------------ */
#define DYLD_INTERPOSE(_replacement, _replacee) \
    static __attribute__((used)) __attribute__((section("__DATA,__interpose"))) \
    struct { const void *replacement; const void *replacee; } \
    _interpose_##_replacee = { \
        (const void *)(unsigned long)&(_replacement), \
        (const void *)(unsigned long)&(_replacee)  \
    }
#endif

#ifdef _WIN32
__attribute__((constructor))
#endif
static void guardian_init(void)
{
    DBG("dylib caricata. socket=%s",
        getenv("GUARDIAN_SOCKET") ? getenv("GUARDIAN_SOCKET") : "(non impostato)");
}

/* ---------- comunicazione con il daemon ------------------------------ */

static int guardian_check(const char *path, char *const argv[])
{
#ifdef _WIN32
    // Su Windows, per ora, facciamo un bypass totale
    return 0; 
#else
    if (getenv("GUARDIAN_BYPASS") != NULL) return 0;

    const char *sock_path = getenv("GUARDIAN_SOCKET");
    if (sock_path == NULL || sock_path[0] == '\0') return 0;

    DBG("hook: %s", path);

    char command[4096] = {0};
    strlcpy(command, path, sizeof(command));
    if (argv != NULL) {
        for (int i = 1; argv[i] != NULL; i++) {
            strlcat(command, " ", sizeof(command));
            strlcat(command, argv[i], sizeof(command));
        }
    }

    char escaped[8192] = {0};
    int j = 0;
    for (int i = 0; command[i] && j < (int)sizeof(escaped) - 2; i++) {
        if (command[i] == '"' || command[i] == '\\')
            escaped[j++] = '\\';
        escaped[j++] = command[i];
    }

    char payload[8192];
    snprintf(payload, sizeof(payload),
        "{\"command\":\"%s\",\"work_dir\":\"\",\"agent_name\":\"\"}\n",
        escaped);

    int fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (fd < 0) { DBG("socket() fallita"); return 0; }

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strlcpy(addr.sun_path, sock_path, sizeof(addr.sun_path));

    if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        close(fd);
        DBG("connect() fallita (%s)", sock_path);
        return 0;
    }

    if (write(fd, payload, strlen(payload)) < 0) {
        close(fd);
        return 0;
    }

    char response[2048] = {0};
    ssize_t n = read(fd, response, sizeof(response) - 1);
    close(fd);

    if (n <= 0) return 0;

    if (strstr(response, "\"decision\":\"block\"") != NULL) {
        fprintf(stderr, "guardian: bloccato dalla policy\n");
        return 1;
    }
    return 0;
#endif
}

/* ---------- sostituzioni --------------------------------------------- */

#ifndef _WIN32
static int guardian_execve(const char *path, char *const argv[], char *const envp[])
{
    if (guardian_check(path, argv)) {
        errno = EPERM;
        return -1;
    }
    return execve(path, argv, envp);
}

static int guardian_posix_spawn(
    pid_t *pid, const char *path,
    const posix_spawn_file_actions_t *fa,
    const posix_spawnattr_t *attr,
    char *const argv[], char *const envp[])
{
    if (guardian_check(path, argv)) {
        errno = EPERM;
        return EPERM;
    }
    return posix_spawn(pid, path, fa, attr, argv, envp);
}

static int guardian_posix_spawnp(
    pid_t *pid, const char *file,
    const posix_spawn_file_actions_t *fa,
    const posix_spawnattr_t *attr,
    char *const argv[], char *const envp[])
{
    if (guardian_check(file, argv)) {
        errno = EPERM;
        return EPERM;
    }
    return posix_spawnp(pid, file, fa, attr, argv, envp);
}

/* ---------- dichiarazioni DYLD_INTERPOSE ----------------------------- */

DYLD_INTERPOSE(guardian_execve,       execve);
DYLD_INTERPOSE(guardian_posix_spawn,  posix_spawn);
DYLD_INTERPOSE(guardian_posix_spawnp, posix_spawnp);
#endif	