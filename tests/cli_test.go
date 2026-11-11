// Package tests checks the extensions in this repository against a Context
// Circuit CLI named by CC_CLI. It tests only this repository: every manifest
// through the CLI, each script against sample input, and the published text.
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// extensions are the folders at the repository root that hold an extension.
var extensions = []string{"verifier", "token-usage"}

var cliPath string

func TestMain(m *testing.M) {
	cliPath = os.Getenv("CC_CLI")
	if cliPath == "" {
		fmt.Fprintln(os.Stderr, "CC_CLI is not set: point it at a Context Circuit CLI binary, the release that the manifests' requires_cli names or later")
		os.Exit(1)
	}
	out, err := exec.Command(cliPath, "version").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "CC_CLI=%s does not run: %v\n", cliPath, err)
		os.Exit(1)
	}
	version := strings.TrimSpace(string(out))
	for _, ext := range extensions {
		need := strings.TrimPrefix(manifestField(ext, "requires_cli"), ">=")
		if need != "" && compareVersions(version, need) < 0 {
			fmt.Fprintf(os.Stderr, "CC_CLI is %s, and %s requires %s or later\n", version, ext, need)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

// repoRoot is the repository root, one level above this package.
func repoRoot() string {
	dir, _ := os.Getwd()
	return filepath.Dir(dir)
}

// manifestField reads a top-level scalar from an extension's manifest.
func manifestField(ext, key string) string {
	data, err := os.ReadFile(filepath.Join(repoRoot(), ext, "extension.yaml"))
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*"?([^"\n]*)"?\s*$`)
	if m := re.FindSubmatch(data); m != nil {
		return strings.TrimSpace(string(m[1]))
	}
	return ""
}

func compareVersions(a, b string) int {
	pa, pb := strings.Split(strings.SplitN(a, "-", 2)[0], "."), strings.Split(strings.SplitN(b, "-", 2)[0], ".")
	for i := 0; i < 3; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// cli runs the CLI in dir and returns stdout, stderr, and the exit code.
func cli(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(cliPath, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("running CLI %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), code
}

// mustCLI runs the CLI and fails the test on a non-zero exit.
func mustCLI(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, errOut, code := cli(t, dir, args...)
	if code != 0 {
		t.Fatalf("CLI %v exited %d:\n%s\n%s", args, code, out, errOut)
	}
	return out
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// remote builds a bare repository holding this checkout's extension folders,
// as they are on disk, with each extension tagged at its manifest version.
func remote(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	src := filepath.Join(base, "src")
	for _, ext := range extensions {
		copyDir(t, filepath.Join(repoRoot(), ext), filepath.Join(src, ext))
	}
	git(t, src, "init", "-q", "-b", "main")
	git(t, src, "add", "-A")
	git(t, src, "commit", "-q", "-m", "extensions")
	for _, ext := range extensions {
		git(t, src, "tag", ext+"/v"+manifestField(ext, "version"))
	}
	bare := filepath.Join(base, "remote.git")
	git(t, base, "init", "-q", "--bare", bare)
	git(t, src, "push", "-q", bare, "main", "--tags")
	url := filepath.ToSlash(bare)
	if !strings.HasPrefix(url, "/") {
		url = "/" + url // a Windows drive path: file:///C:/...
	}
	return "file://" + url
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, _ := d.Info()
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
}

// workspace makes a throwaway workspace pinned to the CLI under test, with
// both extensions added from a local remote at their tags.
func workspace(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "work space ~")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustCLI(t, dir, "init", "--name", "Test", "--purpose", "Extension tests", "--member", "tester", "--member-name", "Tester")
	version := strings.TrimSpace(mustCLI(t, dir, "version"))
	if err := os.WriteFile(filepath.Join(dir, ".context-circuit", "CLI_VERSION"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	url := remote(t)
	for _, ext := range extensions {
		mustCLI(t, dir, "extension", "add", "--id", ext, "--url", url, "--ref", ext+"/v"+manifestField(ext, "version"), "--subpath", ext)
	}
	return dir
}

// plan adds a repository with one commit, a plan for it, and its worktree.
func plan(t *testing.T, ws string) (id, worktree string) {
	t.Helper()
	mustCLI(t, ws, "repo", "init", "--id", "app", "--path", "repositories/app", "--base", "main")
	app := filepath.Join(ws, "repositories", "app")
	if err := os.WriteFile(filepath.Join(app, "README.md"), []byte("app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, app, "add", "-A")
	git(t, app, "commit", "-q", "-m", "start")
	var created struct{ ID string }
	decode(t, mustCLI(t, ws, "--json", "record", "create", "--kind", "plan", "--slug", "fix", "--title", "Fix", "--repo", "app"), &created)
	var prepared struct{ Path string }
	decode(t, mustCLI(t, ws, "--json", "worktree", "prepare", "--repo", "app", "--plan", created.ID), &prepared)
	return created.ID, prepared.Path
}

func decode(t *testing.T, text string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(text), v); err != nil {
		t.Fatalf("decoding %q: %v", text, err)
	}
}

func TestManifestsThroughCLI(t *testing.T) {
	ws := workspace(t)
	mustCLI(t, ws, "agent", "setup", "--host", "claude-code")
	var result struct {
		OK     bool `json:"ok"`
		Issues []struct {
			Issue string `json:"issue"`
		} `json:"issues"`
	}
	out, _, _ := cli(t, ws, "--json", "check")
	decode(t, out, &result)
	if !result.OK || len(result.Issues) > 0 {
		t.Fatalf("check is not clean:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(ws, ".claude", "agents", "cc-x-verifier-checker.md")); err != nil {
		t.Fatalf("agent setup did not install the checker role: %v", err)
	}
}

func TestVersionsMatchChangelogs(t *testing.T) {
	heading := regexp.MustCompile(`(?m)^## (\S+)`)
	for _, ext := range extensions {
		version := manifestField(ext, "version")
		if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(version) {
			t.Errorf("%s: version %q is not X.Y.Z", ext, version)
		}
		data, err := os.ReadFile(filepath.Join(repoRoot(), ext, "CHANGELOG.md"))
		if err != nil {
			t.Errorf("%s: %v", ext, err)
			continue
		}
		m := heading.FindSubmatch(data)
		if m == nil || string(m[1]) != version {
			t.Errorf("%s: the newest CHANGELOG.md heading is not ## %s", ext, version)
		}
	}
}

func TestTagsMatchManifests(t *testing.T) {
	root := repoRoot()
	if _, err := exec.Command("git", "-C", root, "rev-parse", "--git-dir").Output(); err != nil {
		t.Skip("not a Git checkout")
	}
	for _, ext := range extensions {
		out, err := exec.Command("git", "-C", root, "tag", "--list", ext+"/v*").Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, tag := range strings.Fields(string(out)) {
			manifest, err := exec.Command("git", "-C", root, "show", tag+":"+ext+"/extension.yaml").Output()
			if err != nil {
				t.Errorf("%s: no %s/extension.yaml at that tag", tag, ext)
				continue
			}
			want := "version: " + strings.TrimPrefix(tag, ext+"/v")
			if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(want) + `\s*$`).Match(manifest) {
				t.Errorf("%s: the manifest at that tag does not say %q", tag, want)
			}
		}
	}
}

func TestPublishedText(t *testing.T) {
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`/Users/|/Volumes/|/home/[a-z]|[A-Z]:\\Users\\`),
		regexp.MustCompile(`\b(p\d{4}|i\d{3})\b`),
		regexp.MustCompile(`(?i)co-authored-by|generated (with|by) `),
	}
	root := repoRoot()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || path == filepath.Join(root, "tests")) {
			return filepath.SkipDir
		}
		ext := filepath.Ext(path)
		if d.IsDir() || (ext != ".md" && ext != ".yaml" && ext != ".yml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, re := range forbidden {
			if loc := re.Find(data); loc != nil {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s: published text contains %q", rel, loc)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
