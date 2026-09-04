package contract

import (
	"strconv"
	"strings"

	"github.com/syst3mctl/godoclive/internal/model"
)

// responseSet collects the responses a handler can produce, collapsing the ones
// that say the same thing.
//
// Deduplicating by status code alone — which is what this replaced — loses a
// real distinction. A handler that answers 200 with a full article or, given
// ?format=summary, with a trimmed one, documents two payloads under one status;
// keeping only whichever branch the walk reached first tells a client to expect
// a shape it may never receive, with nothing to say the other exists.
//
// Two rules keep the set honest rather than merely larger. Responses agreeing
// on both status and payload collapse, so a status written from four branches
// appears once. And a body-less response is dropped next to a documented
// payload for the same status: `w.WriteHeader(400); return` in one branch and
// an error body in another describe the same 400, and admitting "no body" as a
// second alternative would document a response the handler never sends.
type responseSet struct {
	responses []model.ResponseDef
	seen      map[string]bool
	bodied    map[int]bool
}

func newResponseSet() *responseSet {
	return &responseSet{
		seen:   make(map[string]bool),
		bodied: make(map[int]bool),
	}
}

// add records a response unless the set already says the same thing.
func (s *responseSet) add(r model.ResponseDef) {
	key := strconv.Itoa(r.StatusCode) + "|" + bodyKey(r.Body)
	if s.seen[key] {
		return
	}

	if r.Body == nil {
		if s.bodied[r.StatusCode] {
			return
		}
	} else if !s.bodied[r.StatusCode] {
		// The first payload for this status supersedes any body-less entry
		// recorded for it earlier.
		s.dropBodyless(r.StatusCode)
	}

	s.seen[key] = true
	if r.Body != nil {
		s.bodied[r.StatusCode] = true
	}
	s.responses = append(s.responses, r)
}

// dropBodyless removes the body-less entry for a status, if one was recorded.
func (s *responseSet) dropBodyless(code int) {
	for i, r := range s.responses {
		if r.StatusCode == code && r.Body == nil {
			s.responses = append(s.responses[:i], s.responses[i+1:]...)
			delete(s.seen, strconv.Itoa(code)+"|")
			return
		}
	}
}

// has reports whether the set already carries a response for a status.
func (s *responseSet) has(code int) bool {
	for _, r := range s.responses {
		if r.StatusCode == code {
			return true
		}
	}
	return false
}

// all returns the collected responses in the order they were found.
func (s *responseSet) all() []model.ResponseDef {
	return s.responses
}

// bodyKey renders a payload's identity, so that two responses carrying the same
// shape compare equal and two carrying different shapes do not.
func bodyKey(td *model.TypeDef) string {
	if td == nil {
		return ""
	}
	var b strings.Builder
	writeBodyKey(&b, td, 0)
	return b.String()
}

// maxBodyKeyDepth bounds the walk. A type reaching this deep is either
// pathological or recursive, and its identity is already established by the
// levels above.
const maxBodyKeyDepth = 6

func writeBodyKey(b *strings.Builder, td *model.TypeDef, depth int) {
	if td == nil || depth > maxBodyKeyDepth {
		b.WriteString("…")
		return
	}
	if td.IsPointer {
		b.WriteByte('*')
	}
	b.WriteString(string(td.Kind))
	b.WriteByte(':')
	if td.Package != "" {
		b.WriteString(td.Package)
		b.WriteByte('.')
	}
	b.WriteString(td.Name)
	if td.Elem != nil {
		b.WriteByte('[')
		writeBodyKey(b, td.Elem, depth+1)
		b.WriteByte(']')
	}
	// An anonymous struct — the shape standing in for a gin.H literal — has no
	// name, so its fields are its identity.
	if td.Kind == model.KindStruct && td.Name == "" {
		b.WriteByte('{')
		for i, f := range td.Fields {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(f.JSONName)
			b.WriteByte(' ')
			writeBodyKey(b, &f.Type, depth+1)
		}
		b.WriteByte('}')
	}
}
