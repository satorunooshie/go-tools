package main

import (
	"golang.org/x/tools/custom/analyzer/nosprintf"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(
		nosprintf.Analyzer,
	)
}
