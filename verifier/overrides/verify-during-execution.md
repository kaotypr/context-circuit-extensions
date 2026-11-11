# Verify during execution

This workspace allowed the verifier to check each implementation once, before
the implementation is reported.

1. After the last commit in the worktree, dispatch `x-verifier-checker` with
   `--override verify-during-execution` and the worktree's path.
2. Put its `Verdict`, `Findings` and `Not checked` in your report, beside what
   was implemented and tested.
3. Fix nothing because of it. The person reads the findings and decides what
   happens next.
