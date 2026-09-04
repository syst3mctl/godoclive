package extractor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestHasIgnoreDirective(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "preceding line",
			src:  "package p\n//godoclive:ignore\nvar x = 1\n",
			want: true,
		},
		{
			name: "trailing comment",
			src:  "package p\nvar x = 1 //godoclive:ignore\n",
			want: true,
		},
		{
			name: "skip variant",
			src:  "package p\n//godoclive:skip\nvar x = 1\n",
			want: true,
		},
		{
			name: "no directive",
			src:  "package p\n// regular comment\nvar x = 1\n",
			want: false,
		},
		{
			name: "directive with spaces",
			src:  "package p\n// godoclive:ignore\nvar x = 1\n",
			want: true,
		},
		{
			// A blank line between the directive and the node breaks adjacency,
			// so the directive must NOT apply. This is what stops one directive
			// from leaking onto a second, densely-packed statement below it.
			name: "blank line above — not adjacent",
			src:  "package p\n//godoclive:ignore\n\nvar x = 1\n",
			want: false,
		},
		{
			name: "directive plus contiguous doc comment",
			src:  "package p\n//godoclive:ignore\n// docs\nvar x = 1\n",
			want: true,
		},
		{
			name: "three lines above — too far",
			src:  "package p\n//godoclive:ignore\n\n\nvar x = 1\n",
			want: false,
		},
		{
			name: "unrelated directive",
			src:  "package p\n//go:generate foo\nvar x = 1\n",
			want: false,
		},
		{
			// A registration whose handler is an inline func literal spans
			// several lines, and the trailing directive lands after the
			// closing brace — the node's last line, not its first.
			name: "trailing comment on the node's last line",
			src:  "package p\nvar x = []int{\n\t1,\n} //godoclive:ignore\n",
			want: true,
		},
		{
			// Interior lines do not count, so a directive written against
			// something nested inside the node does not suppress the node.
			name: "comment on an interior line",
			src:  "package p\nvar x = []int{\n\t1, //godoclive:ignore\n}\n",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}

			// Find the GenDecl (var x = 1) position.
			var targetPos, targetEnd token.Pos
			ast.Inspect(file, func(n ast.Node) bool {
				if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.VAR {
					targetPos, targetEnd = gd.Pos(), gd.End()
					return false
				}
				return true
			})
			if !targetPos.IsValid() {
				t.Fatal("could not find var declaration in test source")
			}

			got := hasIgnoreDirective(fset, file, targetPos, targetEnd)
			if got != tt.want {
				t.Errorf("hasIgnoreDirective() = %v, want %v", got, tt.want)
			}
		})
	}
}
