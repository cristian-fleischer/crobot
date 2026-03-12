## Ideas:
-CRoBot
 - multi agent, context retrieval, ast-aware diff parsing, multi-pass reviews and more... https://chatgpt.com/share/69b277c8-c69c-800e-b3d9-75a7a73e7645

- add features that help actually fix PR comments
  - from the PR get the branch
  - automatically switch to that branch or create a worktree 
  - go through CR comments, determine if needs fixing, and if required fix them, one commit per fix (easily reversible)
  - comment / resolve the PR comment

- MaintenanceBot - MOBot (BOBot? Bug Obliterator - from WALL-E MO - Microbe-Obliterator)
  - read logs
  - parse and detect patterns / same error
  - create tasks in jira
- WorkerBeeBot
  - create branch
  - fix
  - create PR
