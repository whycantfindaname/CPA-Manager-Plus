# Remove approved CPA Manager Plus dead code

## Goal

清理批准的 S6 孤立前端模块和内部 Go helper，保持当前 Accounts、inspection、usage 与 xAI reset 行为。

## Authority and background

[批准计划 S6](/Users/jasonliao/Desktop/code/Artifacts/infra-dead-code-audit-20261003/DELETION_PLAN.md) 是完整对象清单和保留语义依据。“可以请继续”与后续“确认”已分别满足计划实施、任务创建/启动审批。交付仅 source、未提交；当前状态见 [baseline.json](/Users/jasonliao/Desktop/code/Artifacts/infra-dead-code-cleanup-20261003/task-preparation/baseline.json)。

## Requirements

- R1 删除这七个孤立模块：`apps/web/src/components/common/SplashScreen.tsx`、`entities/usageService/baseResolver.ts`、`hooks/index.ts`、`hooks/useApi.ts`、`hooks/useDebounce.ts`、`hooks/usePagination.ts`、`pages/PluginStorePage.tsx`（后六项同以 `apps/web/src/` 为根）。经当前引用确认独占后移除 `SplashScreen.scss`；共享品牌资源保留。baseResolver 必要语义转入当前 service-base 入口测试；仅旧自测可删除。
- R2 删除 `apps/manager-server/internal/` 下批准的 11 个符号：`repository/quotasnapshot/lifecycle.go:preferredAccuracy`、`repository/usagemonitoring/filter.go:rawStatsConditions`、`model/codex_inspection.go:valueOrLower`、`service/codexinspection/service.go:forceFinalizeInspectionRun, verifySourceFileStatusTarget, deleteAuthFileOnly, deleteAuthFile, patchAuthFile, formatCodexResetLabel`、`service/codexinspection/xai_probe.go:xaiFailureDetails`、`worker/rate_limit_auto_disable.go:xaiFreeUsageResetTimeFromJSONText`。
- R3 范围包含上述本体、仅服务它们的依赖以及直接受影响现有测试；保留当前 WithContext、storedStatsConditions、formatCodexResetLabelAt、xaiResetTimeFromJSONText、PluginsPage 和 feature 内活跃 hooks。已有 Accounts 合并功能与两种产品模式保持。
- R4 不退役活跃 authFiles API、inspection 重定向、owner/API/兼容接口，不修改数据库迁移、生成面板、部署和凭据。不 stage/commit/push/fetch、服务启停或真实 provider 请求。编辑前回读并保留并发修改。
- R5 当前引用复查后，主代理确认删除独占依赖 `service/codexinspection/service.go:doCPAAction`，将旧 `TestDoCPAActionRejectsLargeBusinessFailureResponse` 的 1 MiB `denied` 断言迁入活跃 `service/cpaauthfiles/client_test.go:ValidateActionResponse` 测试；旧 transport wrapper 的 HTTP status 断言随旧路径退役。
- R6 当前引用复查后，主代理确认删除仅服务旧 `usePagination` 的 `apps/web/src/types/common.ts:PaginationState`，保留 barrel、其余通用类型及 feature 内同名泛型；`.trellis/spec/cpa-manager-plus-web/frontend/hook-guidelines.md` 仅移除过时 `useDebounce.ts` 示例。最终 tracked source/spec 范围为 20 个路径，完整映射见清理证据 `cpamp/path-map.json`。

## Acceptance Criteria

- [ ] R1 七模块及确认独占的样式/旧自测移除；当前服务解析的必要断言保留；共享资源和当前页面正常。
- [ ] R2 全部批准内部符号移除，无旧链残留；R3 当前 inspection finalization、usage 统计与 xAI reset 解析断言通过。
- [ ] `npm run type-check`、`npm run lint`、`npm run test`、`npm run build`、`npm run manager-server:test` 与 backend race 检查通过；Go 格式和 `git diff --check` 通过。
- [ ] Full Docker / CPA Panel 现有语义、并发修改和非任务文件保留；结果 source-only/uncommitted，失败如实报告。

## Settled decisions

复杂、跨 frontend/backend 清理，配套 design 与 implement。当前只完成 planning，由主代理审阅后执行已授权启动。
