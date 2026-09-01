# CPA Manager Plus Managed Sync Errors

## CPAMP_FRONTEND_VERIFY_FAILED

- Stage: project workflow.
- Meaning: the repository frontend test command failed or timed out.
- Inspect: locate the first failing web or repository architecture test.
- Action: fix the owning frontend or test code and rerun
  `npm run test -- --maxWorkers=1 --testTimeout=20000`.
- Stop condition: backend verification and later operations remain blocked.

## CPAMP_BACKEND_VERIFY_FAILED

- Stage: project workflow.
- Meaning: `npm run manager-server:test` failed or timed out.
- Inspect: locate the first failing Manager Server package and test.
- Action: fix the owning Go code or test and rerun the backend command.
- Stop condition: do not build, migrate, package, or deploy until both source
  verification stages pass.
