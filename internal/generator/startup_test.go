package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/syst3mctl/godoclive/internal/pipeline"
)

func TestGeneratedStartupUsesAnalyzedAPI(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is required to execute generated JavaScript")
	}
	eps, err := pipeline.RunPipeline(e2eTestdataDir("chi-basic"), "./...", nil)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "startup.cjs")
	// Execute the real data selection from the generated scripts in document order.
	// Rendering the DOM is not necessary to observe which endpoint set the UI selects.
	const js = `const fs = require('fs'), vm = require('vm'), assert = require('assert');
 const base = process.argv[2] + '/';
 const html = fs.readFileSync(base + 'index.html', 'utf8');
 const ctx = {window:{}}; vm.createContext(ctx);
 for (const m of html.matchAll(/<script([^>]*)>([\s\S]*?)<\/script>/g)) {
   const code = m[1].includes('src="app.js"') ? fs.readFileSync(base+'app.js','utf8') : m[2];
   const marker = 'var data = window.API_DATA || SAMPLE_DATA;';
   if (code.includes(marker)) {
     vm.runInContext(code.slice(0, code.indexOf(marker)) + 'window.selected = window.API_DATA || SAMPLE_DATA;})();', ctx);
   } else if (code.includes('window.API_DATA =')) { vm.runInContext(code, ctx); }
 }
 assert.strictEqual(ctx.window.selected.projectName, 'Audit regression');
 assert.strictEqual(ctx.window.selected.endpoints.length, 6);
 assert.strictEqual(ctx.window.selected, ctx.window.API_DATA);
 `
	if err := os.WriteFile(script, []byte(js), 0600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"single", "folder"} {
		t.Run(format, func(t *testing.T) {
			out := t.TempDir()
			if err := Generate(eps, GeneratorConfig{OutputPath: out, Format: format, Title: "Audit regression"}); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command(node, script, out).CombinedOutput(); err != nil {
				t.Fatalf("startup: %v\n%s", err, output)
			}
		})
	}
}
