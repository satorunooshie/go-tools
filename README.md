# Go Tools

This repository provides [gopls](https://pkg.go.dev/golang.org/x/tools/gopls) with custom analyzers.

## Usage
### Install gopls
```sh
$ cd ./go-tools
$ make gopls
```

### Install commands
```sh
$ cd ./go-tools
$ make cmd
```

## How to add custom analyzers

1. Implement the Analyzer in golang.org/x/tools/custom/analyzer
2. Add 1 to golang.org/x/tools/gopls/internal/settings/custom.go
3. Add the command line tool of 1 to cmd

> **Warning**
> If you change files other than the above, you may conflict with upstream
