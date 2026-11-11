package tests

import (
	"strings"
	"testing"
)

func TestVerifierDispatch(t *testing.T) {
	ws := workspace(t)
	mustCLI(t, ws, "agent", "setup", "--host", "claude-code")
	id, worktree := plan(t, ws)
	before := git(t, worktree, "status", "--porcelain")
	head := git(t, worktree, "rev-parse", "HEAD")

	dispatch := func(args ...string) (string, int) {
		base := []string{"--json", "agent", "dispatch", "--host", "claude-code", "--plan", id, "--path", worktree}
		out, errOut, code := cli(t, ws, append(base, args...)...)
		return out + errOut, code
	}

	var brief struct {
		AgentType string `json:"agent_type"`
		ReadOnly  bool   `json:"read_only"`
		Prompt    string `json:"prompt"`
	}
	out, code := dispatch("--role", "x-verifier-checker")
	if code != 0 {
		t.Fatalf("dispatching the checker on request failed:\n%s", out)
	}
	decode(t, out, &brief)
	if brief.AgentType != "cc-x-verifier-checker" || !brief.ReadOnly {
		t.Errorf("checker brief: agent_type %q, read_only %v; want cc-x-verifier-checker, read-only", brief.AgentType, brief.ReadOnly)
	}
	last := -1
	for _, heading := range []string{"Verdict", "Findings", "Not checked"} {
		i := strings.LastIndex(brief.Prompt, heading)
		if i < 0 || i < last {
			t.Errorf("the brief does not list the return heading %q in order", heading)
		}
		last = i
	}

	steps := []struct {
		name  string
		setup []string
		args  []string
		ok    bool
	}{
		{"verify without an allowance", nil, []string{"--role", "x-verifier-checker", "--override", "verify-during-execution"}, false},
		{"loop without an allowance", nil, []string{"--role", "x-verifier-checker", "--override", "repair-loop", "--round", "1"}, false},
		{"verify once allowed", []string{"--allow", "verify-during-execution"}, []string{"--role", "x-verifier-checker", "--override", "verify-during-execution"}, true},
		{"loop round 1 once allowed", []string{"--allow", "repair-loop", "--rounds", "2"}, []string{"--role", "x-verifier-checker", "--override", "repair-loop", "--round", "1"}, true},
		{"fix in round 1", nil, []string{"--role", "worker", "--task", "fix the findings", "--override", "repair-loop", "--round", "1"}, true},
		{"verify in round 2", nil, []string{"--role", "x-verifier-checker", "--override", "repair-loop", "--round", "2"}, true},
		{"fix in the last round", nil, []string{"--role", "worker", "--task", "fix the findings", "--override", "repair-loop", "--round", "2"}, false},
		{"round past the limit", nil, []string{"--role", "x-verifier-checker", "--override", "repair-loop", "--round", "3"}, false},
	}
	for _, step := range steps {
		if step.setup != nil {
			mustCLI(t, ws, append([]string{"extension", "override", "--id", "verifier"}, step.setup...)...)
		}
		out, code := dispatch(step.args...)
		if (code == 0) != step.ok {
			t.Errorf("%s: exit %d, want accepted=%v:\n%s", step.name, code, step.ok, out)
		}
	}

	if _, _, code := cli(t, ws, "extension", "override", "--id", "verifier", "--allow", "repair-loop", "--rounds", "4"); code == 0 {
		t.Error("an allowance above the manifest's 3 rounds was accepted")
	}
	if after := git(t, worktree, "status", "--porcelain"); after != before {
		t.Errorf("the worktree changed:\n%s", after)
	}
	if git(t, worktree, "rev-parse", "HEAD") != head {
		t.Error("the worktree has a new commit")
	}
}
