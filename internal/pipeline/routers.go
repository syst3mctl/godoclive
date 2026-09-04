package pipeline

import (
	"github.com/syst3mctl/godoclive/internal/detector"
	"github.com/syst3mctl/godoclive/internal/extractor"
)

// RawRouteSource is one extracted route tagged with the framework that found
// it. A project on more than one router produces routes from several
// extractors, and the endpoint keeps that provenance so the docs can say which
// framework serves it.
type RawRouteSource struct {
	Route  extractor.RawRoute
	Router detector.RouterKind
}

// extractorFor returns the extractor for a framework, or nil when the kind has
// none.
func extractorFor(kind detector.RouterKind) extractor.Extractor {
	switch kind {
	case detector.RouterKindChi:
		return &extractor.ChiExtractor{}
	case detector.RouterKindGin:
		return &extractor.GinExtractor{}
	case detector.RouterKindStdlib:
		return &extractor.StdlibExtractor{}
	case detector.RouterKindGorilla:
		return &extractor.GorillaExtractor{}
	case detector.RouterKindEcho:
		return &extractor.EchoExtractor{}
	case detector.RouterKindFiber:
		return &extractor.FiberExtractor{}
	}
	return nil
}

// dedupeRoutes drops routes two extractors both claimed.
//
// The extractors are type-guarded and normally partition the registrations
// between them, but a handler mounted across frameworks — a chi router given
// to http.Handle, say — is visible to both. Identity is the registration site
// itself (file and line) plus the method and path it produced, so a genuine
// second registration of the same path elsewhere in the file is kept.
func dedupeRoutes(routes []RawRouteSource) []RawRouteSource {
	if len(routes) < 2 {
		return routes
	}
	type key struct {
		method, path, file string
		line               int
	}
	seen := make(map[key]bool, len(routes))
	out := routes[:0:0]
	for _, r := range routes {
		k := key{r.Route.Method, r.Route.Path, r.Route.File, r.Route.Line}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}
