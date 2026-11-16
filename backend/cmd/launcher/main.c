#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <libgen.h>
#include <limits.h>
#include <sys/stat.h>

int main(int argc, char *argv[]) {
    // 1. Unset GODEBUG to prevent Go 1.24+ runtime panic on UGOS Pro
    unsetenv("GODEBUG");

    // 2. Resolve self path
    char self_path[PATH_MAX];
    ssize_t len = readlink("/proc/self/exe", self_path, sizeof(self_path) - 1);
    if (len == -1) {
        strncpy(self_path, argv[0], sizeof(self_path) - 1);
        self_path[sizeof(self_path) - 1] = '\0';
    } else {
        self_path[len] = '\0';
    }

    // 3. Resolve binary directory and app root
    char self_dir_buf[PATH_MAX];
    strncpy(self_dir_buf, self_path, sizeof(self_dir_buf) - 1);
    self_dir_buf[sizeof(self_dir_buf) - 1] = '\0';
    char *bin_dir = dirname(self_dir_buf);

    char app_root_buf[PATH_MAX];
    strncpy(app_root_buf, bin_dir, sizeof(app_root_buf) - 1);
    app_root_buf[sizeof(app_root_buf) - 1] = '\0';
    char *app_root = dirname(app_root_buf);

    // Switch to app root directory
    if (app_root && strlen(app_root) > 0) {
        chdir(app_root);
    }

    // 4. Construct target binary path
    char core_path[PATH_MAX];
    snprintf(core_path, sizeof(core_path), "%s/accounting_core", bin_dir);

    // 5. Ensure core binary is executable
    chmod(core_path, 0755);

    // 6. Execute core Go binary with original arguments
    execv(core_path, argv);

    // Fallback search in current working directory
    execv("bin/accounting_core", argv);
    execv("./bin/accounting_core", argv);

    // If execv reaches here, it failed
    fprintf(stderr, "[FATAL] accounting_serv launcher failed to execute '%s'\n", core_path);
    perror("execv");
    return 1;
}
