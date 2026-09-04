package model_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
)

// docOf parses a source snippet and returns the parsed doc for its single
// function declaration.
func docOf(t *testing.T, src string) model.HandlerDoc {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "x.go", "package p\n\n"+src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			return model.ParseHandlerDoc(fn.Doc, fn.Name.Name)
		}
	}
	t.Fatal("no function declaration in snippet")
	return model.HandlerDoc{}
}

func TestParseHandlerDoc(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		wantSummary string
		wantDesc    string
		wantDeprec  bool
	}{
		{
			name:        "leading identifier is dropped and the rest capitalized",
			src:         "// ListUsers returns every user in the account.\nfunc ListUsers() {}",
			wantSummary: "Returns every user in the account.",
		},
		{
			name:        "first sentence is the summary, the whole comment the description",
			src:         "// GetUser returns one user. It 404s when the id is unknown.\nfunc GetUser() {}",
			wantSummary: "Returns one user.",
			wantDesc:    "It 404s when the id is unknown.",
		},
		{
			name:        "paragraphs are joined and inner wrapping collapsed",
			src:         "// CreateUser registers an account.\n//\n// The email must be\n// unique.\nfunc CreateUser() {}",
			wantSummary: "Registers an account.",
			wantDesc:    "The email must be unique.",
		},
		{
			name:        "a deprecation notice is flagged, not described",
			src:         "// OldList lists users.\n//\n// Deprecated: use ListUsers.\nfunc OldList() {}",
			wantSummary: "Lists users.",
			wantDeprec:  true,
		},
		{
			name:        "a comment not opening with the identifier is left alone",
			src:         "// Returns the health of the process.\nfunc Healthz() {}",
			wantSummary: "Returns the health of the process.",
		},
		{
			name:        "a decimal does not end the sentence",
			src:         "// Ping speaks v1.2 of the protocol. Nothing else.\nfunc Ping() {}",
			wantSummary: "Speaks v1.2 of the protocol.",
			wantDesc:    "Nothing else.",
		},
		{
			name:        "a lower-case word after a period continues the sentence",
			src:         "// Fetch calls e.g. the upstream service.\nfunc Fetch() {}",
			wantSummary: "Calls e.g. the upstream service.",
		},
		{
			name:        "directives never reach the output",
			src:         "//godoclive:ignore\n// Debug dumps state.\nfunc Debug() {}",
			wantSummary: "Dumps state.",
		},
		{
			name: "no comment yields nothing",
			src:  "func Bare() {}",
		},
		{
			name:       "a comment that is only a deprecation notice",
			src:        "// Deprecated: removed in v2.\nfunc Gone() {}",
			wantDeprec: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := docOf(t, tt.src)
			if got.Summary != tt.wantSummary {
				t.Errorf("Summary = %q, want %q", got.Summary, tt.wantSummary)
			}
			if got.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", got.Description, tt.wantDesc)
			}
			if got.Deprecated != tt.wantDeprec {
				t.Errorf("Deprecated = %v, want %v", got.Deprecated, tt.wantDeprec)
			}
		})
	}
}

// The description must never be a verbatim echo of the summary.
func TestParseHandlerDoc_SingleSentenceHasNoDescription(t *testing.T) {
	got := docOf(t, "// Healthz reports liveness.\nfunc Healthz() {}")
	if got.Description != "" {
		t.Errorf("Description = %q, want empty for a one-sentence comment", got.Description)
	}
	if !strings.HasPrefix(got.Summary, "Reports") {
		t.Errorf("Summary = %q", got.Summary)
	}
}
