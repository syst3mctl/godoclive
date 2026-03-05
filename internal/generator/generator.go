package generator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/syst3mctl/godoclive/internal/model"
)

// GeneratorConfig holds configuration for documentation generation.
type GeneratorConfig struct {
	OutputPath string // Output directory or file path (default: ./docs)
	Format     string // "folder" or "single" (default: "folder")
	Title      string // Project title displayed in docs
	Version    string // Project version displayed in docs
	BaseURL    string // Pre-fill base URL in Try It
	Theme      string // "light" or "dark" (default: "light")
}

// Generate transforms analyzed endpoints into documentation output.
func Generate(endpoints []model.EndpointDef, cfg GeneratorConfig) error {
	if cfg.OutputPath == "" {
		cfg.OutputPath = "./docs"
	}
	if cfg.Format == "" {
		cfg.Format = "folder"
	}
	if cfg.Theme == "" {
		cfg.Theme = "light"
	}

	apiData := buildAPIData(endpoints, cfg)
	jsonBytes, err := json.Marshal(apiData)
	if err != nil {
		return fmt.Errorf("marshaling API data: %w", err)
	}

	switch cfg.Format {
	case "single":
		return generateSingle(cfg.OutputPath, jsonBytes, cfg.Theme)
	default:
		return generateFolder(cfg.OutputPath, jsonBytes, cfg.Theme)
	}
}

// apiData is the JSON structure injected as window.API_DATA.
type apiData struct {
	ProjectName string        `json:"projectName"`
	Version     string        `json:"version"`
	BaseURL     string        `json:"baseUrl"`
	Endpoints   []apiEndpoint `json:"endpoints"`
}

type apiEndpoint struct {
	Method      string        `json:"method"`
	Path        string        `json:"path"`
	Summary     string        `json:"summary"`
	Tag         string        `json:"tag"`
	Tags        []string      `json:"tags,omitempty"`
	HandlerName string        `json:"handlerName"`
	Deprecated  bool          `json:"deprecated"`
	Auth        apiAuth       `json:"auth"`
	Params      []apiParam    `json:"params"`
	Headers     []apiHeader   `json:"headers"`
	Body        *apiBody      `json:"body"`
	Responses   []apiResponse `json:"responses"`
	Unresolved  []string      `json:"unresolved"`
}

type apiAuth struct {
	Required bool     `json:"required"`
	Schemes  []string `json:"schemes"`
}

type apiParam struct {
	Name     string  `json:"name"`
	In       string  `json:"in"`
	Type     string  `json:"type"`
	Required bool    `json:"required"`
	Default  *string `json:"default,omitempty"`
	Example  string  `json:"example,omitempty"`
}

type apiHeader struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type apiBody struct {
	ContentType string     `json:"contentType"`
	TypeName    string     `json:"typeName"`
	Fields      []apiField `json:"fields"`
	Example     string     `json:"example"`
}

type apiField struct {
	Name     string `json:"name"`
	JSONName string `json:"jsonName"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Example  string `json:"example,omitempty"`
}

type apiResponse struct {
	Status      int        `json:"status"`
	Description string     `json:"description"`
	ContentType string     `json:"contentType"`
	Source      string     `json:"source"`
	Fields      []apiField `json:"fields,omitempty"`
	Example     string     `json:"example,omitempty"`
}

// buildAPIData converts []EndpointDef into the JSON structure the UI expects.
func buildAPIData(endpoints []model.EndpointDef, cfg GeneratorConfig) apiData {
	title := cfg.Title
	if title == "" {
		title = "API Documentation"
	}
	ver := cfg.Version
	if ver == "" {
		ver = "v1.0.0"
	}

	ad := apiData{
		ProjectName: title,
		Version:     ver,
		BaseURL:     cfg.BaseURL,
		Endpoints:   make([]apiEndpoint, 0, len(endpoints)),
	}

	for _, ep := range endpoints {
		ae := apiEndpoint{
			Method:      ep.Method,
			Path:        ep.Path,
			Summary:     ep.Summary,
			HandlerName: ep.HandlerName,
			Deprecated:  ep.Deprecated,
			Unresolved:  ep.Unresolved,
			Auth:        convertAuth(ep.Auth),
			Params:      convertParams(ep.Request),
			Headers:     convertHeaders(ep.Request.Headers),
			Body:        convertBody(ep.Request),
			Responses:   convertResponses(ep.Responses),
		}
		if ae.Unresolved == nil {
			ae.Unresolved = []string{}
		}
		// Preserve all tags; use first as primary for backward compat.
		if len(ep.Tags) > 0 {
			ae.Tag = ep.Tags[0]
			ae.Tags = ep.Tags
		}
		ad.Endpoints = append(ad.Endpoints, ae)
	}

	return ad
}

func convertAuth(a model.AuthDef) apiAuth {
	schemes := make([]string, len(a.Schemes))
	for i, s := range a.Schemes {
		schemes[i] = string(s)
	}
	return apiAuth{Required: a.Required, Schemes: schemes}
}

func convertParams(req model.RequestDef) []apiParam {
	var params []apiParam
	for _, p := range req.PathParams {
		params = append(params, apiParam{
			Name: p.Name, In: "path", Type: p.Type,
			Required: p.Required, Default: p.Default, Example: p.Example,
		})
	}
	for _, p := range req.QueryParams {
		params = append(params, apiParam{
			Name: p.Name, In: "query", Type: p.Type,
			Required: p.Required, Default: p.Default, Example: p.Example,
		})
	}
	return params
}

func convertHeaders(headers []model.ParamDef) []apiHeader {
	var out []apiHeader
	for _, h := range headers {
		out = append(out, apiHeader{Name: h.Name, Type: h.Type, Required: h.Required})
	}
	return out
}

func convertBody(req model.RequestDef) *apiBody {
	if req.Body == nil {
		return nil
	}
	ct := req.ContentType
	if ct == "" {
		ct = "application/json"
	}
	b := &apiBody{
		ContentType: ct,
		TypeName:    req.Body.Name,
		Fields:      convertTypeDefFields(req.Body),
	}
	if req.Body.Example != nil {
		exBytes, err := json.MarshalIndent(req.Body.Example, "", "  ")
		if err == nil {
			b.Example = string(exBytes)
		}
	} else if req.Body.Kind == model.KindStruct && len(req.Body.Fields) > 0 {
		if obj := buildStructExample(req.Body); obj != nil {
			exBytes, err := json.MarshalIndent(obj, "", "  ")
			if err == nil {
				b.Example = string(exBytes)
			}
		}
	}
	return b
}

func buildStructExample(td *model.TypeDef) map[string]interface{} {
	obj := make(map[string]interface{})
	for _, f := range td.Fields {
		if f.JSONName == "" || f.JSONName == "-" {
			continue
		}
		obj[f.JSONName] = f.Example
	}
	if len(obj) == 0 {
		return nil
	}
	return obj
}

func convertTypeDefFields(td *model.TypeDef) []apiField {
	if td == nil || td.Kind != model.KindStruct {
		return nil
	}
	var fields []apiField
	for _, f := range td.Fields {
		if f.JSONName == "" || f.JSONName == "-" {
			continue
		}
		af := apiField{
			Name:     f.Name,
			JSONName: f.JSONName,
			Type:     typeDefToString(&f.Type),
			Required: f.Required,
		}
		if f.Example != nil {
			exBytes, err := json.Marshal(f.Example)
			if err == nil {
				af.Example = string(exBytes)
			}
		}
		fields = append(fields, af)
	}
	return fields
}

func convertResponses(responses []model.ResponseDef) []apiResponse {
	var out []apiResponse
	for _, r := range responses {
		ar := apiResponse{
			Status:      r.StatusCode,
			Description: r.Description,
			ContentType: r.ContentType,
			Source:      r.Source,
		}
		if r.Body != nil {
			ar.Fields = convertTypeDefFields(r.Body)
			if r.Body.Example != nil {
				exBytes, err := json.MarshalIndent(r.Body.Example, "", "  ")
				if err == nil {
					ar.Example = string(exBytes)
				}
			}
		}
		out = append(out, ar)
	}
	return out
}

// typeDefToString produces a human-readable type string for display.
func typeDefToString(td *model.TypeDef) string {
	if td == nil {
		return "unknown"
	}
	switch td.Kind {
	case model.KindStruct:
		if td.Name != "" {
			return td.Name
		}
		return "object"
	case model.KindSlice:
		if td.Elem != nil {
			return "[]" + typeDefToString(td.Elem)
		}
		return "[]unknown"
	case model.KindMap:
		return "object"
	case model.KindPrimitive:
		return td.Name
	case model.KindInterface:
		return "any"
	default:
		return td.Name
	}
}

// generateFolder writes index.html, style.css, app.js, and fonts/ to outputDir.
func generateFolder(outputDir string, apiJSON []byte, theme string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Write index.html with injected API_DATA.
	html := string(GetHTML())
	html = injectAPIData(html, apiJSON)
	html = injectTheme(html, theme)
	if err := os.WriteFile(filepath.Join(outputDir, "index.html"), []byte(html), 0o644); err != nil {
		return fmt.Errorf("writing index.html: %w", err)
	}

	// Write style.css with font-face declarations pointing to local files.
	css := fontFaceDecls(false) + "\n" + string(GetCSS())
	if err := os.WriteFile(filepath.Join(outputDir, "style.css"), []byte(css), 0o644); err != nil {
		return fmt.Errorf("writing style.css: %w", err)
	}

	// Write app.js.
	if err := os.WriteFile(filepath.Join(outputDir, "app.js"), GetJS(), 0o644); err != nil {
		return fmt.Errorf("writing app.js: %w", err)
	}

	// Write fonts.
	fontsDir := filepath.Join(outputDir, "fonts")
	if err := os.MkdirAll(fontsDir, 0o755); err != nil {
		return fmt.Errorf("creating fonts directory: %w", err)
	}
	for _, name := range FontNames() {
		data := GetFont(name)
		if data != nil {
			if err := os.WriteFile(filepath.Join(fontsDir, name), data, 0o644); err != nil {
				return fmt.Errorf("writing font %s: %w", name, err)
			}
		}
	}

	return nil
}

// generateSingle writes a single self-contained index.html.
func generateSingle(outputDir string, apiJSON []byte, theme string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	html := string(GetHTML())

	// Inline CSS (with base64 font-face).
	css := fontFaceDecls(true) + "\n" + string(GetCSS())
	html = strings.Replace(html, `<link rel="stylesheet" href="style.css">`,
		"<style>\n"+css+"\n</style>", 1)

	// Inline JS.
	html = strings.Replace(html, `<script src="app.js"></script>`,
		"<script>\n"+string(GetJS())+"\n</script>", 1)

	// Inject API data and theme.
	html = injectAPIData(html, apiJSON)
	html = injectTheme(html, theme)

	if err := os.WriteFile(filepath.Join(outputDir, "index.html"), []byte(html), 0o644); err != nil {
		return fmt.Errorf("writing index.html: %w", err)
	}

	return nil
}

// injectAPIData inserts a <script> tag with window.API_DATA before app.js so
// the data is available when app.js executes.
func injectAPIData(html string, apiJSON []byte) string {
	tag := "<script>window.API_DATA = " + string(apiJSON) + ";</script>\n"
	// Insert before app.js so data is available when app.js runs.
	if strings.Contains(html, `<script src="app.js">`) {
		return strings.Replace(html, `<script src="app.js">`, tag+`<script src="app.js">`, 1)
	}
	// Fallback for single-file mode (no external app.js).
	return strings.Replace(html, "</body>", tag+"</body>", 1)
}

// injectTheme replaces the default theme in the flash-free loading script.
func injectTheme(html string, theme string) string {
	return strings.Replace(html,
		"var t = localStorage.getItem('gdl-theme') || 'dark';",
		"var t = localStorage.getItem('gdl-theme') || '"+theme+"';",
		1)
}

// fontFaceDecls generates @font-face CSS for the embedded fonts.
// If inline is true, uses base64 data URIs; otherwise, uses relative paths.
func fontFaceDecls(inline bool) string {
	type fontSpec struct {
		family string
		weight string
		file   string
	}
	fonts := []fontSpec{
		{"Inter", "400", "inter-latin-400.woff2"},
		{"Inter", "600", "inter-latin-600.woff2"},
		{"JetBrains Mono", "400", "jetbrains-mono-latin-400.woff2"},
	}

	var sb strings.Builder
	for _, f := range fonts {
		var src string
		if inline {
			data := GetFont(f.file)
			if data == nil {
				continue
			}
			b64 := base64.StdEncoding.EncodeToString(data)
			src = "url(data:font/woff2;base64," + b64 + ") format('woff2')"
		} else {
			src = "url(fonts/" + f.file + ") format('woff2')"
		}
		sb.WriteString("@font-face {\n")
		sb.WriteString("  font-family: '" + f.family + "';\n")
		sb.WriteString("  font-style: normal;\n")
		sb.WriteString("  font-weight: " + f.weight + ";\n")
		sb.WriteString("  font-display: swap;\n")
		sb.WriteString("  src: " + src + ";\n")
		sb.WriteString("}\n")
	}
	return sb.String()
}

// Serve starts an HTTP server serving the generated docs at the given address.
func Serve(dir, addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(dir)))
	fmt.Printf("Serving docs at http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

// ServeWithSSE starts an HTTP server with an SSE /events endpoint for live reload.
// The returned channel can be used to push reload events.
func ServeWithSSE(dir, addr string) (chan struct{}, error) {
	reloadCh := make(chan struct{}, 1)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(dir)))
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		for {
			select {
			case <-reloadCh:
				_, _ = fmt.Fprintf(w, "event: reload\ndata: {}\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	go func() {
		fmt.Printf("Serving docs at http://localhost%s\n", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		}
	}()

	return reloadCh, nil
}
