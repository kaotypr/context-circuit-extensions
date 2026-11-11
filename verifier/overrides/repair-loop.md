# Repair loop

This workspace allowed an implement, verify, fix loop with the verifier.

- **Clean** means the checker's `Verdict` is `clean`.
- **What to fix first:** failing checks, then anything that misses the plan's
  success criteria, then the remaining findings.
- **What is not fixed:** a finding outside the plan's outcome is reported, not
  fixed. A finding marked `outcome` stops the loop: give it to the person with
  the round it came from.
- **Each round:** dispatch `x-verifier-checker` with `--override repair-loop
  --round N`. On findings with rounds left, dispatch a worker with the findings
  as its task and the same round, then the checker again with the next round.
  Each fix is its own commit on the worktree branch.
- **The report:** say how it ended, clean after N rounds or the findings left
  at the limit, and name each fix's commit.
