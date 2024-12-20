package main

import (
	"golang.org/x/tools/custom/analyzer/nosprintf"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(nosprintf.Analyzer) }
