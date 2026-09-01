# CPA Manager Plus Managed Sync

This directory is the single in-repository entry for the CPA Manager Plus
managed workflow. The initial contract targets `lwj_dev`; Agent Infra owns
repository convergence. Project verification runs the repository frontend
tests followed by the Manager Server Go tests.

```bash
npm run test -- --maxWorkers=1 --testTimeout=20000
npm run manager-server:test
```

The frontend suite is deliberately limited to one Vitest worker on the shared
Linux workspace. The default parallel worker count can turn metadata latency
into per-test timeouts even when the same tests pass in isolation.

Builds, packaging, data migration acceptance, deployment, and service restart
remain explicitly triggered project or platform operations. The initial
contract does not modify runtime data. See [errors.md](errors.md) after a
failure.
