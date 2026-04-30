#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
    #include <process.h>
    #include <windows.h>
    /* Su Windows definiamo le macro per simulare il comportamento di waitpid */
    #define WIFEXITED(status) (1)
    #define WEXITSTATUS(status) (status)
#else
    #include <unistd.h>
    #include <sys/wait.h>
    #include <sys/types.h>
#endif

int main(int argc, char *argv[])
{
    if (argc < 2) {
        fprintf(stderr, "usage: exec-helper <cmd> [args...]\n");
        return 1;
    }

#ifdef _WIN32
    /* 
     * Windows non ha fork(). 
     * Usiamo _spawnvp che combina la creazione del processo e l'esecuzione.
     * _P_WAIT blocca il padre finché il figlio non termina, simulando waitpid.
     */
    intptr_t status = _spawnvp(_P_WAIT, argv[1], (const char* const*)&argv[1]);
    
    if (status == -1) {
        perror("_spawnvp");
        return 1;
    }
    return (int)status;

#else
    /* Codice originale per Linux/macOS */
    pid_t pid = fork();
    if (pid < 0) {
        perror("fork");
        return 1;
    }

    if (pid == 0) {
        /* processo figlio: esegue il comando via execvp */
        execvp(argv[1], &argv[1]);
        perror("execvp");
        _exit(1);
    }

    /* processo padre: attende il figlio */
    int status;
    waitpid(pid, &status, 0);
    return WIFEXITED(status) ? WEXITSTATUS(status) : 1;
#endif
}