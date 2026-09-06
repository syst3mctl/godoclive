package action_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func actionScripts(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var action struct {
		Runs struct {
			Steps []struct {
				ID  string `yaml:"id"`
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(data, &action); err != nil {
		t.Fatal(err)
	}
	scripts := make(map[string]string)
	for _, step := range action.Runs.Steps {
		scripts[step.ID] = step.Run
	}
	return scripts
}

func TestActionAnalysisReuseAndCoverageGate(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required for GitHub Action shell tests")
	}
	scripts := actionScripts(t)
	for _, tc := range []struct {
		name, docs, spec, min string
		fail                  bool
		want                  string
	}{
		{"both", "site output", "api spec.json", "75", false, "generate"},
		{"docs", "site output", "", "", false, "generate"},
		{"spec", "", "api spec.json", "", false, "openapi"},
		{"coverage only", "", "", "", false, ""},
		{"gate failure", "site", "api.json", "90", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "calls")
			// JSON argv records preserve argument boundaries, including paths with spaces.
			mock := `#!/usr/bin/env bash
set -euo pipefail
jq -cn --args '$ARGS.positional' -- "$@" >> "$CALL_LOG"
if [ "$1" = validate ]; then
  printf '%s\n' '{"total":4,"resolved":3,"partial":1,"coverage":75,"endpoints":[{"method":"GET","path":"/items","unresolved":["response body: unresolved"]}]}'
fi
`
			if err := os.WriteFile(filepath.Join(dir, "godoclive"), []byte(mock), 0700); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "outputs")
			env := append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "CALL_LOG="+log, "GITHUB_OUTPUT="+output, "GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary"), "GODOCLIVE_PACKAGES=./...", "GODOCLIVE_MIN_COVERAGE="+tc.min, "GODOCLIVE_DOCS="+tc.docs, "GODOCLIVE_OPENAPI="+tc.spec, "GODOCLIVE_DOCS_FORMAT=single", "GODOCLIVE_WORKING_DIRECTORY=service")
			run := func(id string) ([]byte, error) {
				cmd := exec.Command("bash", "-c", scripts[id])
				cmd.Env = env
				cmd.Dir = dir
				return cmd.CombinedOutput()
			}
			out, err := run("validate")
			if tc.fail {
				if err == nil || !strings.Contains(string(out), "below the required 90%") {
					t.Fatalf("gate did not fail: %v %s", err, out)
				}
			} else {
				if err != nil {
					t.Fatalf("validate: %v %s", err, out)
				}
				if !strings.Contains(string(out), "GET /items -> response body: unresolved") {
					t.Fatalf("missing derived issue summary: %s", out)
				}
				if out, err = run("spec"); err != nil {
					t.Fatalf("outputs: %v %s", err, out)
				}
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
			count := 1
			if tc.want != "" {
				count++
			}
			if len(lines) != count {
				t.Fatalf("analysis calls = %d, want %d: %s", len(lines), count, calls)
			}
			if !strings.HasPrefix(lines[0], `["validate","--json"`) {
				t.Fatalf("unexpected coverage call: %s", lines[0])
			}
			if tc.want != "" && !strings.HasPrefix(lines[1], `["`+tc.want+`"`) {
				t.Fatalf("wrong generation command: %s", lines[1])
			}
			if tc.docs != "" && tc.spec != "" && !tc.fail && !strings.Contains(lines[1], `"--openapi","api spec.json"`) {
				t.Fatalf("spec not shared with HTML generation: %s", lines[1])
			}
			emitted, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(emitted), "coverage=75\nendpoints=4\nunresolved=1") {
				t.Fatalf("action outputs changed: %s", emitted)
			}
			if tc.spec != "" && !tc.fail && !strings.Contains(string(emitted), "path=service/"+tc.spec) {
				t.Fatalf("missing spec output: %s", emitted)
			}
		})
	}
}
