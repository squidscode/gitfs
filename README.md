# GitFS

GitFS is a utility that allows automatic commits and automatic pushes of git repositories reachable from a root directory. This utility is useful for programmers that want their online git repositories to mirror their current working directory. This is particularly useful if you add this script to your cron jobs. 

I will be adding GitFS to my cron jobs to automatically commit and push code for repos I want to keep up-to-date (e.g. a repo that holds my scripts / problem sets / notes).

GitFS can also track multiple projects since it walks the root directory searching for git repositories and `.gitfs` files, which holds repo-specific auto-commit and auto-push information.

## .gitfs files

`.gitfs` files must be stored next to `.git` directories. Specify the gitfs configuration in JSON:

```json
{
  "autocommit": false,
  "autopush": false,
  "commit-message": "...",
  "remote": "origin",
  "branch": "wip"
}
```

All fields are **optional** and will **default to the values specified above**. You can also override the default fields by adding command line flags to the call to the gitfs binary:

```
Gitfs tracks all projects in a root directories and
auto-commits all changes based on a ".gitfs" config file

Usage: gitfs ROOT_DIR [-d/--depth DEPTH] [-m/--message MESSAGE]
           [-v/--verbose] [-b/--branch BRANCH] [-r/--remote REMOTE]
           [--auto-commit] [--auto-push] [-h/--help]

    ROOT_DIR - the root directory
    DEPTH - the depth of the walk (default: 5)
    MESSAGE - the commit message
    BRANCH - the branch to commit to (default: wip)
    REMOTE - the remote git repo to push to (default: origin)
```
