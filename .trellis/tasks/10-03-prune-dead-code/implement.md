# CPA Manager Plus execution plan

1. 主代理审阅产物/manifests 后启动；复查身份、当前 Git/dirty、目标内容与所有引用，明确直接测试/独占依赖范围。
2. 清理七个前端模块和确认独占的 SplashScreen 样式；保留共享资源；baseResolver 有用语义移到当前入口测试。跑相应 web 测试。
3. 清理 Go 纯 helper，再清 inspection 无入口操作链；保留 WithContext finalization、当前 usage filter、reset parser 与现有行为测试。跑相应 Go 包测试。
4. 根目录执行 `npm run type-check`、`npm run lint`、`npm run test`、`npm run build`、`npm run manager-server:test`；从 `apps/manager-server/` 执行 `go test -race ./...`。用 `gofmt -l <changed Go files>` 检查实际改动格式。
5. 若 repoSourceIntegrity 缺少 `origin/main...HEAD`，按规范使用真实、预期的本地 diff base 设置 `CPA_MANAGER_CHANGED_FILES_BASE` 并记录依据；不得为了通过而虚构 base。
6. 根目录 `git diff --check`，定向旧符号/模块引用复查，检查两种模式、当前 Accounts 与 shared resource diff。主代理验收全范围；必要 spec 更新有依据才做。按 source-only/uncommitted 决定收尾，无 stage/commit/push/activation。

检查缺失/失败报告实际原因，不降低验收条件。无需 fetch、真实凭据、CPA queue、provider 请求或服务启动。三组各自作为回滚点，仅局部撤销任务差异并保留并发内容。
