---
title: 账号巡检（Codex / xAI）
description: 使用 CPA Manager Plus 在本地或 Manager Server 定时巡检 Codex 和 xAI 账号，查看配额、凭证、workspace、billing 证据与安全动作。
---

# 账号巡检（Codex / xAI）

账号巡检用于判断账号为什么不能稳定服务请求。当前页面路由和部分 UI 仍保留 `Codex Inspection` 名称，但巡检目标已经覆盖 Codex 和 xAI。

打开[账号巡检演示](https://seakee.github.io/CPA-Manager-Plus/#/demo/codex-inspection)可以查看虚构结果，不会向 Provider 发送请求。

如果只是排查某一条请求，先看[请求监控](./monitoring.md)；确认问题集中在账号、凭证或额度后再进入巡检。

## 本地与服务端巡检

- **本机巡检**：由当前浏览器会话执行，适合少量账号和临时诊断。
- **服务端巡检**：由 Manager Server 执行，支持定时任务、历史、日志和统一动作策略。

服务端巡检前确认 CPA URL、CPA Management Key、Auth File 和稳定 `auth_index` 均可用。

## Codex 检查内容

- 账号计划、5 小时/周额度窗口、reset 和剩余额度。
- OAuth Token 与认证状态。
- Workspace 是否停用或不可用。
- `usage_limit_reached` 等明确额度证据。
- 是否建议保留、重新授权、禁用、启用或删除。

缺失字段保持未知，不会被当作健康或异常。

## Manager 主机上的 Codex 登录

服务端巡检页还可以直接读取 Manager Server 主机上的 Codex 登录。Manager
Server 启动本机 `codex app-server`，通过官方的 `account/read`、
`account/rateLimits/read` 和 `account/usage/read` RPC 获取当前登录身份、额度窗口
和每日 token 活动。这条路径不要求账号登记在 CPA，也不会读取或返回 OAuth
令牌。

默认使用 `PATH` 中的 `codex`。原生服务找不到命令时，可将
`CPAMP_CODEX_EXECUTABLE` 设置为 Codex CLI 的绝对路径。该区域读取的是 Manager
Server 所在主机，不是打开网页的浏览器主机；远程 Manager Server 没有 Codex
CLI 或未登录时会显示不可用，CPA 凭证巡检仍可继续使用。

一个 app-server 进程只代表一个 `CODEX_HOME` 登录。当前版本自动读取 Manager
主机的默认登录；CPA 中的其他账号仍由原有凭证巡检覆盖。每日 token 是账号活动
总量，不含模型、输入/输出、缓存或 service tier 拆分，不能据此计算 API 等价
成本或 weekly pool 美元估值。

## xAI 检查内容

xAI 巡检优先使用不发送模型推理请求的只读证据：

- Grok Build / CLI OAuth 可查询时读取周额度、月度账单和账号状态。
- 免费额度耗尽事件可进入受控的滚动 24 小时冷却。
- 付费 `api.x.ai` OAuth 无法访问 CLI billing 时，可使用只读身份接口确认官方 API 身份。
- 身份检查成功只代表凭证可访问身份接口，不代表具体模型、聊天路由、费用或剩余额度已经验证。
- 不明确的 `403`、地区限制或模型权限不会被统一解释为凭证失效。

## 结果和动作

- **保留**：没有足够证据要求处理。
- **重新授权**：OAuth 或认证状态明确失效。
- **人工复核**：证据不足或可能涉及权限、地区和模型范围。
- **禁用**：账号当前不适合参与新请求，并且策略允许。
- **启用**：由同一巡检自动化禁用且已明确恢复。
- **删除**：只有账号明确失效、文件不再共享且用户确认时执行。

不要只看动作名称，要同时查看 Provider、原因代码、脱敏证据和最近请求表现。

## 定时巡检与自动化边界

服务端可按间隔或每天指定时间运行。建议先使用仅记录或保守动作模式，观察结果后再启用自动禁用。

所有自动恢复遵循“谁禁用、谁恢复”：

- 巡检只恢复由巡检自己禁用的凭证。
- 配额冷却只恢复对应冷却记录禁用的凭证。
- 手动禁用和账号处理队列禁用不会被巡检越权恢复。

## 与其他页面的关系

- 配额窗口和冷却：[凭证管理](./accounts.md)
- OAuth 重新授权：[OAuth 登录](./oauth.md)
- 凭证状态：[凭证管理](./accounts.md)
- 反复认证失败：[账号处理队列](./account-actions.md)
- 单条失败证据：[请求监控](./monitoring.md)
