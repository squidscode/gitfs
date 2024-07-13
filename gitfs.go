package main

import (
	"fmt"
	"os"
    "os/exec"
	"path/filepath"
	"slices"
    "encoding/json"
	// "strings"
)

/*
 * The driver for the gitfs tool
 */
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

    println("Scanning ", dir)

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
    dat := getDefaultGitfsConfig()
    contents, err := os.ReadFile(filepath.Join(dir, ".gitfs")) 
    if err != nil {
        println(".gitfs file found!")
        if err := json.Unmarshal(contents, &dat); err != nil {
            panic(err)
        }
    }

    out, _ := exec.Command("bash", "-c", "cd "+dir+"; git diff").Output()
    if len(out) == 0 {
        println("Command on", dir, "exited without any git diff output")
    } else {

    }
}

func getDefaultGitfsConfig() map[string]any {
    return map[string]any {
        "autocommit": true,
        "branch": "master",
    }
}

func printHelp() {
    fmt.Printf(
`gitfs tracks all projects in a root directories and
auto-commits all changes based on a ".gitfs" config file

Usage: gitfs DIR_PATH

    DIR_PATH - the directory to walk and report git-diffs
`)
}

func check(err error) {
    if err != nil {
        panic(err)
    }
}
