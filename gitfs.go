package main

import (
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "slices"
    "strings"
)

type argument_options_t struct {
    depth int
    commit_message string
    verbose bool
}

// global state for argument options because they should only be set by the driver
var argument_options argument_options_t = argument_options_t{
    depth:5, 
    commit_message:"this commit was automatically committed by gitfs",
    verbose:true,
}

func main() {
    if len(os.Args) != 2 {
        fmt.Fprintf(os.Stderr, "Invalid number of arguments")
        printHelp()
        os.Exit(1)
    }

    _, err := os.ReadDir(os.Args[1])
    check(err)

    scanDir(os.Args[1], 5)
}

func scanDir(dir string, max_depth int) {
    if max_depth == 0 {
        return
    }

    c, err := os.ReadDir(dir)
    check(err)

    println("Scanning", dir)

    var subdirs []string
    for _, entry := range c {
        if entry.IsDir() {
            subdirs = append(subdirs, entry.Name())
        }
    }

    if slices.Contains(subdirs, ".git") {
        println("Found root:", dir)
        processDirectory(dir)
    } else {
        for _, s := range subdirs {
            scanDir(filepath.Join(dir, s), max_depth - 1)
        }
    }

}

func processDirectory(dir string) {
    config := getDefaultGitfsConfig()
    gitfs := filepath.Join(dir, ".gitfs")
    if _, err := os.Stat(gitfs); err == nil {
        contents, err := os.ReadFile(gitfs)
        check(err)
        processGitFsFile(dir, contents, &config)
    } else {
        println("No `.gitfs` file found for", dir)
        println("Resorting to default configuration")
    }

    git_diff := outputBashInDir(dir, "git diff")
    if len(git_diff) != 0 && config["autocommit"].(bool) { // stuff to commit found
        cur_git_branch := strings.TrimSpace(
            string(outputBashInDir(dir, "git branch --show-current")),
        )
        stdout, stderr, err := runBashInDir(
            dir, 
            fmt.Sprintf(
                "set -e;                   "+ // IMPORTANT: exit on first error!
                "git stash push;           "+
                "git checkout %s;          "+
                "git stash apply;          "+
                "git add .;                "+
                "git commit -m \"%s\";     "+
                getPushCommand(&config)+"; "+
                "git checkout %s;          "+
                "git stash pop",
                config["branch"],
                config["commit-message"],
                cur_git_branch,
            ),
        )
        if argument_options.verbose || err != nil {
            println("STDOUT:")
            println(string(stdout))
            println("STDERR:")
            println(string(stderr))
        }
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
    ex := exec.Command(
        "bash",
        "-c",
        fmt.Sprintf("cd %s; %s", dir, cmd),
    )
    stdout_reader, err := ex.StdoutPipe(); check(err)
    stderr_reader, err := ex.StderrPipe(); check(err)
    exit_error := ex.Start()
    stdout_buf := []byte{}
    stderr_buf := []byte{}
    outn, err := stdout_reader.Read(stdout_buf); check(err)
    errn, err := stderr_reader.Read(stderr_buf); check(err)
    ex.Wait()
    println(outn, errn)
    return stdout_buf, stderr_buf, exit_error
}

func getPushCommand(config *map[string]any) string {
    if (*config)["autopush"].(bool) {
        println("Autopush enabled!")
        return fmt.Sprintf("git push %s %s", (*config)["remote"], (*config)["branch"])
    } else {
        return ":"
    }
}

func processGitFsFile(dir string, contents []byte, default_config *map[string]any) {
    println(".gitfs file found!")
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
        if _, exists := json_data[k]; !exists {
            // log the error
            println("[ERROR] key", k, "is not a valid `gitfs` configuration!")
        }
    }
}

func getDefaultGitfsConfig() map[string]any {
    return map[string]any {
        "autocommit": false, // should gitfs autocommit any changes
        "autopush": false, // should gitfs automatically push if an origin is specified
        "commit-message": argument_options.commit_message, // the commit message
        "remote": "origin", // which remote to push to (ie. `git push ????`)
        "branch": "main", // which branch should be committed to
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
        `gitfs tracks all projects in a root directories and
        auto-commits all changes based on a ".gitfs" config file

        Usage: gitfs ROOT_DIR [-d/--depth DEPTH]

        ROOT_DIR - the root directory
        DEPTH - the depth of the walk (default is 5)
        `)
}

func check(err error) {
    if err != nil {
        panic(err)
    }
}
