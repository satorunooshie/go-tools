package settings

import (
	"golang.org/x/tools/custom/analyzer/nosprintf"
	"golang.org/x/tools/gopls/internal/protocol"
)

func addCustomAnalyzers(a []*Analyzer) []*Analyzer {
	return append(a, []*Analyzer{
		{
			analyzer:    nosprintf.Analyzer,
			enabled:     true,
			actionKinds: []protocol.CodeActionKind{protocol.SourceFixAll, protocol.QuickFix},
		},
	}...)
}
