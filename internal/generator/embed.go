package generator

import "embed"

//go:embed ui/index.html
var embeddedHTML []byte

//go:embed ui/style.css
var embeddedCSS []byte

//go:embed ui/app.js
var embeddedJS []byte

//go:embed ui/fonts
var embeddedFonts embed.FS

// GetHTML returns the embedded index.html content.
func GetHTML() []byte { return embeddedHTML }

// GetCSS returns the embedded style.css content.
func GetCSS() []byte { return embeddedCSS }

// GetJS returns the embedded app.js content.
func GetJS() []byte { return embeddedJS }

// GetFont returns the embedded font file by name (e.g. "inter-latin-400.woff2").
// Returns nil if the font is not found.
func GetFont(name string) []byte {
	data, err := embeddedFonts.ReadFile("ui/fonts/" + name)
	if err != nil {
		return nil
	}
	return data
}

// FontNames returns all available embedded font file names.
func FontNames() []string {
	entries, err := embeddedFonts.ReadDir("ui/fonts")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}
