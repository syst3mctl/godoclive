package model

import (
	"go/ast"
	"strings"
	"unicode"
)

// deprecatedPrefix marks a paragraph that go/doc treats as a deprecation
// notice rather than prose about what the function does.
const deprecatedPrefix = "Deprecated:"

// HandlerDoc is the prose a handler's own doc comment contributes to its
// documentation.
type HandlerDoc struct {
	Summary     string // First sentence, with the leading identifier removed
	Description string // The prose after that sentence; empty when there is none
	Deprecated  bool   // A "Deprecated:" paragraph was present
}

// ParseHandlerDoc reads a handler's doc comment into a summary and a
// description.
//
// The comment a Go developer already wrote is a better summary than anything
// splitting the function name on camel-case humps can produce: "ListUsers"
// yields "List Users", while the comment above it says which users and on what
// terms. Name inference stays as the fallback for handlers with no comment.
//
// Go's convention is that the comment opens with the identifier — "ListUsers
// returns every user in the account." Repeating the function name in a
// rendered summary reads as a stutter next to the method and path, so a
// leading identifier is dropped and the next word capitalized.
//
// name is the declared function name; pass "" to leave any leading identifier
// in place. Directive comments (//go:generate, //godoclive:ignore) are removed
// by CommentGroup.Text and never reach the output.
func ParseHandlerDoc(doc *ast.CommentGroup, name string) HandlerDoc {
	if doc == nil {
		return HandlerDoc{}
	}

	var out HandlerDoc
	var prose []string
	for _, para := range splitParagraphs(doc.Text()) {
		if strings.HasPrefix(para, deprecatedPrefix) {
			// go/doc gives this paragraph a defined meaning. It says why the
			// handler is going away, not what it does, and EndpointDef records
			// the fact separately as a flag.
			out.Deprecated = true
			continue
		}
		prose = append(prose, para)
	}
	if len(prose) == 0 {
		return out
	}

	lead := firstSentence(prose[0])
	out.Summary = capitalizeFirst(trimIdentifier(lead, name))

	// The description carries what the summary does not. Repeating the first
	// sentence in both fields shows a reader the same line twice — on the card,
	// where they sit one above the other, and in any OpenAPI viewer.
	var rest []string
	if tail := strings.TrimSpace(strings.TrimPrefix(prose[0], lead)); tail != "" {
		rest = append(rest, tail)
	}
	rest = append(rest, prose[1:]...)
	out.Description = strings.Join(rest, "\n\n")
	return out
}

// splitParagraphs breaks doc text on blank lines and collapses the line breaks
// inside each paragraph, which are an artifact of the comment's wrapping.
func splitParagraphs(text string) []string {
	var paras []string
	for _, block := range strings.Split(text, "\n\n") {
		var lines []string
		for _, line := range strings.Split(block, "\n") {
			if l := strings.TrimSpace(line); l != "" {
				lines = append(lines, l)
			}
		}
		if len(lines) > 0 {
			paras = append(paras, strings.Join(lines, " "))
		}
	}
	return paras
}

// firstSentence returns the leading sentence of a paragraph, terminator
// included.
//
// A period only ends a sentence when what follows looks like the start of the
// next one: whitespace and then an upper-case letter, or the end of the
// paragraph. That keeps "v1.2", "e.g." and "user.Name" inside the sentence
// they belong to.
func firstSentence(para string) string {
	runes := []rune(para)
	for i, r := range runes {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		rest := strings.TrimLeftFunc(string(runes[i+1:]), unicode.IsSpace)
		if rest == "" {
			return strings.TrimSpace(string(runes[:i+1]))
		}
		// No space after the terminator: still inside a token like "v1.2".
		if len(runes) > i+1 && !unicode.IsSpace(runes[i+1]) {
			continue
		}
		if next := []rune(rest)[0]; unicode.IsUpper(next) {
			return strings.TrimSpace(string(runes[:i+1]))
		}
	}
	return strings.TrimSpace(para)
}

// trimIdentifier drops a leading "Name " from a sentence written in Go's doc
// convention. A comment that opens with anything else is left alone.
func trimIdentifier(sentence, name string) string {
	if name == "" {
		return sentence
	}
	rest, ok := strings.CutPrefix(sentence, name)
	if !ok || rest == "" || !unicode.IsSpace(rune(rest[0])) {
		return sentence
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return sentence
	}
	return rest
}

// capitalizeFirst upper-cases the leading letter, leaving the rest untouched so
// that identifiers and acronyms keep their spelling.
func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
