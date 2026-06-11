package extractor

import (
	"go/ast"
	"go/token"
	"strings"
)

// hasIgnoreDirective reports whether the route registration at nodePos is
// annotated with a //godoclive:ignore (or //godoclive:skip) directive. A
// directive is honored when it is either:
//   - a trailing comment on the node's own line, or
//   - in the comment group immediately above the node — i.e. the group's last
//     line is the line directly above nodePos.
//
// Requiring the directive to be *immediately* adjacent (no blank-line gap)
// stops one directive from leaking onto a second statement below it: in a
// densely-packed block of registrations, a wider window would suppress the
// route after the intended one as well.
func hasIgnoreDirective(fset *token.FileSet, file *ast.File, nodePos token.Pos) bool {
	if !nodePos.IsValid() {
		return false
	}
	targetLine := fset.Position(nodePos).Line

	for _, cg := range file.Comments {
		// adjacent: the whole comment group sits directly above the node, so a
		// directive anywhere in it (e.g. directive + a following doc comment)
		// still applies.
		adjacent := fset.Position(cg.End()).Line == targetLine-1
		for _, c := range cg.List {
			if !adjacent && fset.Position(c.Pos()).Line != targetLine {
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

// nestedStmtBodies returns the statement lists nested inside a compound
// statement — if/else, for, range, switch, type-switch, select, labeled, or a
// bare block. Route registrations are frequently guarded by a condition (e.g.
// `if dep != nil { mux.Handle(...) }`) or placed inside a loop, so every walker
// must descend into these bodies. Simple (non-compound) statements return nil.
func nestedStmtBodies(stmt ast.Stmt) [][]ast.Stmt {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		var bodies [][]ast.Stmt
		if s.Body != nil {
			bodies = append(bodies, s.Body.List)
		}
		if s.Else != nil {
			// Else is a *ast.BlockStmt (`else { … }`) or *ast.IfStmt
			// (`else if …`); wrapping it lets the caller recurse uniformly.
			bodies = append(bodies, []ast.Stmt{s.Else})
		}
		return bodies
	case *ast.ForStmt:
		if s.Body != nil {
			return [][]ast.Stmt{s.Body.List}
		}
	case *ast.RangeStmt:
		if s.Body != nil {
			return [][]ast.Stmt{s.Body.List}
		}
	case *ast.BlockStmt:
		return [][]ast.Stmt{s.List}
	case *ast.LabeledStmt:
		return [][]ast.Stmt{{s.Stmt}}
	case *ast.SwitchStmt:
		return clauseBodies(s.Body)
	case *ast.TypeSwitchStmt:
		return clauseBodies(s.Body)
	case *ast.SelectStmt:
		return clauseBodies(s.Body)
	}
	return nil
}

// clauseBodies returns the body of each case/comm clause inside a switch or
// select statement's block.
func clauseBodies(block *ast.BlockStmt) [][]ast.Stmt {
	if block == nil {
		return nil
	}
	var bodies [][]ast.Stmt
	for _, c := range block.List {
		switch clause := c.(type) {
		case *ast.CaseClause:
			bodies = append(bodies, clause.Body)
		case *ast.CommClause:
			bodies = append(bodies, clause.Body)
		}
	}
	return bodies
}
