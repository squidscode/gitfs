package main

import (
    "encoding/json"
    "fmt"
    "os"
    "strconv"
    "os/exec"
    "path/filepath"
    "slices"
    "strings"
    "log"
    "sync"
)

type argument_options_t struct {
    root_dir string
    depth int
    commit_message string
    verbose bool
    remote string
    branch string
    autocommit bool
    autopush bool
}

// global state for argument options because they should only be set by the driver
var argument_options argument_options_t = argument_options_t{
    root_dir:".",
    depth:5, 
    commit_message:"this commit was automatically committed by gitfs",
    verbose:false,
    remote:"origin",
    branch:"wip",
    autocommit: false,
    autopush: false,
}

/*
 * The driver function for gitfs
 */
func main() {
    if len(os.Args) <= 1 {
        fmt.Fprintln(os.Stderr, "Invalid # of arguments");
        printHelp()
        os.Exit(1);
    }
    parseCommandLineArguments()
    _, err := os.ReadDir(os.Args[1])
    check(err)
    log.SetFlags(log.Ldate | log.Ltime)
    var wg sync.WaitGroup
    scanDir(os.Args[1], argument_options.depth, &wg)
    wg.Wait()
}

func parseCommandLineArguments() {
    argument_options.root_dir = os.Args[1]
    for i := 1; i < len(os.Args); i++ {
        switch os.Args[i] {
        case "-d", "--depth":
            i++
            depth, err := strconv.Atoi(os.Args[i])
            check(err)
            argument_options.depth = depth
        case "-m", "--message":
            i++
            argument_options.commit_message = os.Args[i]
        case "-v", "--verbose":
            argument_options.verbose = true
        case "-b", "--branch":
            i++
            argument_options.branch = os.Args[i]
        case "-r", "--remote":
            i++
            argument_options.remote = os.Args[i]
        case "--auto-commit", "--autocommit":
            i++
            argument_options.autocommit = true
        case "--auto-push", "--autopush":
            i++
            argument_options.autopush = true
        case "-h", "--help":
            printHelp()
            os.Exit(0)
        default:
            if i != 1 {
                printHelp()
                os.Exit(1)
            }
        }
    }
}

func scanDir(dir string, max_depth int, wg *sync.WaitGroup) {
    if max_depth == 0 {
        return
    }
    c, err := os.ReadDir(dir)
    check(err)

    var subdirs []string
    for _, entry := range c {
        if entry.IsDir() {
            subdirs = append(subdirs, entry.Name())
        }
    }

    if slices.Contains(subdirs, ".git") {
        log.Printf("Found git directory at %s\n", dir)
        wg.Add(1)
        go func() {
            defer recoverFromProcessDirectory(dir, wg)
            processDirectory(dir)
        }()
    } else {
        for _, s := range subdirs {
            scanDir(filepath.Join(dir, s), max_depth - 1, wg)
        }
    }
}

func recoverFromProcessDirectory(dir string, wg *sync.WaitGroup) {
    wg.Done()
    if r := recover(); r != nil {
        log.Printf("[%s] Recovered from a panic: %s\n", dir, r)
    }
}

func processDirectory(dir string) {
    config := getDefaultGitfsConfig()
    gitfs := filepath.Join(dir, ".gitfs")
    if _, err := os.Stat(gitfs); err == nil {
        contents, err := os.ReadFile(gitfs)
        check(err)
        log.Printf("[%s] `.gitfs` file found!", dir)
        processGitFsFile(dir, contents, &config)
    } else {
        log.Printf("[%s] No `.gitfs` file found\n", dir)
        log.Printf("[%s] Resorting to default configuration\n", dir)
    }

    if config["autocommit"].(bool) { // stuff to commit found
        commitGitDiff(dir, config)
    } else {
        log.Printf("[%s] Autocommit disabled", dir)
    }
}

func commitGitDiff(dir string, config map[string]any) {
    if len(outputBashInDir(dir, "git diff")) == 0 {
        log.Printf("[%s] Nothing to commit!", dir)
    }
    cur_git_branch := strings.TrimSpace(string(outputBashInDir(dir, "git branch --show-current")))
    log.Printf("[%s] Pushing to branch %s\n", dir, config["branch"])
    _, _, err := runBashInDir(dir, fmt.Sprintf("git stash push; git branch -D %s", config["branch"]))
    check(err)
    stdout, stderr, err := runBashInDir(
        dir, 
        fmt.Sprintf(
            "set -e;                        "+ // IMPORTANT: exit on first error!
            "git checkout -b %s;            "+
            "git stash apply;               "+
            "git add .;                     "+
            "git commit -m \"%s\";          "+
            getPushCommand(dir, &config)+"; ",
            config["branch"],
            config["commit-message"],
            ),
        )
    _, _, err = runBashInDir(dir, fmt.Sprintf("git checkout %s; git stash pop", cur_git_branch))
    check(err)
    if argument_options.verbose {
        log.Printf("[%s] STDOUT:\n%s\n", dir, string(stdout))
    }
    if argument_options.verbose || err != nil {
        log.Fatalf("[%s] STDERR:\n%s\n", dir, string(stderr))
    }
}

func outputBashInDir(dir string, cmd string) []byte {
    out, err := exec.Command(
        "bash",
        "-c",
        fmt.Sprintf("cd %s; %s", dir, cmd),
    ).Output()
    check(err)
    return out
}

func runBashInDir(dir string, cmd string) ([]byte, []byte, error) {
    out, ex := exec.Command(
        "bash",
        "-c",
        fmt.Sprintf("cd %s; %s", dir, cmd),
    ).Output()
    if c, ok := ex.(*exec.ExitError); ok {
        return out, c.Stderr, ex
    }
    return out, []byte{}, nil
}

func getPushCommand(dir string, config *map[string]any) string {
    if (*config)["autopush"].(bool) {
        log.Printf("[%s] Autopush enabled!", dir)
        return fmt.Sprintf("git push -u -f %s %s", (*config)["remote"], (*config)["branch"])
    } else {
        log.Printf("[%s] Autopush disabled!", dir)
        return ":"
    }
}

func processGitFsFile(dir string, contents []byte, default_config *map[string]any) {
    addGitFsToGitIgnore(dir)
    var json_data map[string]interface {}
    err := json.Unmarshal(contents, &json_data)
    check(err)
    for k := range *default_config {
        if val, exists := json_data[k]; exists {
            (*default_config)[k] = val
        }
    }
    for k := range json_data {
        if _, exists := (*default_config)[k]; !exists {
            // log the error
            log.Fatalf("[%s] key \"%s\" is not a valid `gitfs` configuration!", dir, k)
        }
    }
}

func getDefaultGitfsConfig() map[string]any {
    return map[string]any {
        "autocommit": argument_options.autocommit, // should gitfs autocommit any changes
        "autopush": argument_options.autopush, // should gitfs automatically push if an origin is specified
        "commit-message": argument_options.commit_message, // the commit message
        "remote": argument_options.remote, // which remote to push to (ie. `git push ????`)
        "branch": argument_options.branch, // which branch should be committed to
    }
}

func addGitFsToGitIgnore(dir string) {
    gitignore := filepath.Join(dir, ".gitignore")
    _, err := os.Stat(gitignore)
    contents, rerr := os.ReadFile(gitignore)
    if err == nil { // .gitignore does not exist
        os.WriteFile(gitignore, 
            []byte(".gitfs\n"), 
            0644,
        )
    } else if rerr == nil && !strings.Contains(string(contents), ".gitfs\n") { 
        // gitignore does not contain ".gitfs"
        os.WriteFile(gitignore, []byte(".gitfs\n"), os.ModeAppend)
    } else { // unknown error!
        check(err)
        check(rerr)
    }
}

func printHelp() {
    fmt.Printf(
        "Gitfs tracks all projects in a root directories and\n"+
        "auto-commits all changes based on a \".gitfs\" config file\n"+
        "\n"+
        "Usage: gitfs ROOT_DIR [-d/--depth DEPTH] [-m/--message MESSAGE]\n"+
        "           [-v/--verbose] [-b/--branch BRANCH] [-r/--remote REMOTE]\n"+
        "           [--auto-commit] [--auto-push] [-h/--help]\n"+
        "\n"+
        "    ROOT_DIR - the root directory\n"+
        "    DEPTH - the depth of the walk (default: 5)\n"+
        "    MESSAGE - the commit message\n"+
        "    BRANCH - the branch to commit to (default: wip)\n"+
        "    REMOTE - the remote git repo to push to (default: origin)\n",
    )
}

func check(err error) {
    if err != nil {
        panic(err)
    }
}
