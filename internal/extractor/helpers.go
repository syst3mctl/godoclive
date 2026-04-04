package extractor

import (
	"go/ast"
	"go/token"
	"strings"
)

// hasIgnoreDirective checks whether the AST node at nodePos has a
// //godoclive:ignore (or //godoclive:skip) comment directive. It checks:
//  1. Trailing comment on the same line as the node
//  2. Comment on the line immediately preceding the node
//  3. Comment two lines above (to allow a blank line or doc comment between)
func hasIgnoreDirective(fset *token.FileSet, file *ast.File, nodePos token.Pos) bool {
	if !nodePos.IsValid() {
		return false
	}
	targetLine := fset.Position(nodePos).Line

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			cline := fset.Position(c.Pos()).Line
			if cline < targetLine-2 || cline > targetLine {
				continue
			}
			text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if text == "godoclive:ignore" || text == "godoclive:skip" {
				return true
			}
		}
	}
	return false
}
