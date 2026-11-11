package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type scriptResult struct {
	Status      string `json:"status"`
	Summary     string `json:"summary"`
	DetailsPath string `json:"details_path"`
}

// runSummary runs the summary script the CLI would run on this OS, with the
// hook input on stdin, and returns its last stdout line, stderr, and success.
func runSummary(t *testing.T, input map[string]any) (scriptResult, string, bool) {
	t.Helper()
	dir := filepath.Join(repoRoot(), "token-usage")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(dir, "scripts", "summarize.ps1"))
	} else {
		cmd = exec.Command("sh", filepath.Join(dir, "scripts", "summarize.sh"))
	}
	stdin, _ := json.Marshal(input)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var result scriptResult
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		if jerr := json.Unmarshal([]byte(last), &result); jerr != nil {
			t.Fatalf("the last stdout line is not JSON: %q", last)
		}
	}
	return result, stderr.String(), err == nil
}

func hookInput(root, record string, settings map[string]any) map[string]any {
	var rec any
	if record != "" {
		rec = map[string]any{"id": record, "kind": "plan", "path": "plans/" + record + "-x/plan.md", "repositories": []string{"app"}}
	}
	return map[string]any{
		"point": "after implement", "workspace_root": root, "member": "tester",
		"extension": map[string]any{"id": "token-usage", "settings": settings},
		"record":    rec, "worktrees": map[string]string{}, "confirmed": false,
	}
}

func writeLog(t *testing.T, root, folder, record string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(), "token-usage", "testdata", "mixed.log"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, filepath.FromSlash(folder))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, record+".log"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// awkwardName is a folder name with characters a script must not trip on:
// spaces, a tilde, an ampersand, and on Unix the angle brackets that JSON
// encoders escape. Windows does not allow angle brackets in a name.
func awkwardName() string {
	if runtime.GOOS == "windows" {
		return "work space ~ & x"
	}
	return "work space ~ & <x>"
}

const mixedSummary = "4 entries, 1 not-available: reviewer 20500, worker 49500, x-verifier-checker 0; lines not read: 3, 7; totals cover only the entries recorded"

func TestTokenUsageScript(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "mixed-summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		folder   string // where the log is written; empty writes none
		settings map[string]any
		record   string
		status   string // ok, skipped, or failed
		summary  string // the summary, or a part of stderr for a failure
	}{
		{"default folder", ".context-circuit/local/token-usage", nil, "p0001", "ok", mixedSummary},
		{"configured folder", "docs/tokens", map[string]any{"folder": "docs/tokens"}, "p0001", "ok", mixedSummary},
		{"no log", "", nil, "p0002", "skipped", "no entries recorded for p0002"},
		{"no record", "", nil, "", "skipped", "no record at this point"},
		{"absolute folder", "", map[string]any{"folder": "/tmp/tokens"}, "p0001", "failed", "is absolute"},
		{"folder leaving the workspace", "", map[string]any{"folder": "../tokens"}, "p0001", "failed", "leaves the workspace"},
		{"folder leaving it further down", "", map[string]any{"folder": "a/../../tokens"}, "p0001", "failed", "leaves the workspace"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), awkwardName())
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			if c.folder != "" {
				writeLog(t, root, c.folder, c.record)
			}
			result, stderr, ok := runSummary(t, hookInput(root, c.record, c.settings))
			if c.status == "failed" {
				if ok || !strings.Contains(stderr, c.summary) {
					t.Fatalf("want a failure saying %q, got ok=%v stderr=%q", c.summary, ok, stderr)
				}
				return
			}
			if !ok {
				t.Fatalf("the script failed: %s", stderr)
			}
			if result.Status != c.status || !strings.HasPrefix(result.Summary, c.summary) {
				t.Fatalf("got %+v, want status %s and summary %q", result, c.status, c.summary)
			}
			if c.status != "ok" {
				return
			}
			want := filepath.Join(root, filepath.FromSlash(c.folder), c.record+"-summary.md")
			if filepath.Clean(filepath.FromSlash(result.DetailsPath)) != filepath.Clean(want) {
				t.Errorf("details_path %q, want %q", result.DetailsPath, want)
			}
			got, err := os.ReadFile(want)
			if err != nil {
				t.Fatal(err)
			}
			normalized := strings.ReplaceAll(string(got), "\r\n", "\n")
			if expect := strings.ReplaceAll(string(golden), "p0001", c.record); normalized != strings.ReplaceAll(expect, "\r\n", "\n") {
				t.Errorf("summary file differs from testdata/mixed-summary.md:\n%s", normalized)
			}
		})
	}
}

func TestTokenUsageThroughCLI(t *testing.T) {
	ws := workspace(t)
	id, _ := plan(t, ws)
	writeLog(t, ws, ".context-circuit/local/token-usage", id)

	type point struct {
		Instructions []struct{ ID string } `json:"instructions"`
		Ran          []struct {
			ID, Status, Summary string
		} `json:"ran"`
		Skipped []struct{ ID, Reason string } `json:"skipped"`
	}
	hooks := func(when string) point {
		var result struct{ Extensions []point }
		decode(t, mustCLI(t, ws, "--json", "extension", "hooks", "--stage", "implement", "--when", when, "--plan", id, "--repo", "app"), &result)
		if len(result.Extensions) != 1 {
			t.Fatalf("want one point, got %+v", result.Extensions)
		}
		return result.Extensions[0]
	}

	before := hooks("before")
	if len(before.Instructions) != 1 || before.Instructions[0].ID != "token-usage" {
		t.Errorf("before implement: want the token-usage instruction, got %+v", before.Instructions)
	}

	notAllowed := hooks("after")
	if len(notAllowed.Ran) != 0 || len(notAllowed.Skipped) != 1 || !strings.Contains(notAllowed.Skipped[0].Reason, "not allowed") {
		t.Errorf("after implement before allow: want one skip for not allowed, got %+v", notAllowed)
	}

	mustCLI(t, ws, "extension", "allow", "--id", "token-usage")
	allowed := hooks("after")
	if len(allowed.Ran) != 1 || allowed.Ran[0].Status != "ok" || allowed.Ran[0].Summary != mixedSummary {
		t.Errorf("after implement once allowed: got %+v", allowed.Ran)
	}
}
