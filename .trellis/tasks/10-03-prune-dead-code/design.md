# CPA Manager Plus cleanup design

## Boundaries and contracts

按 [批准计划 S6](/Users/jasonliao/Desktop/code/Artifacts/infra-dead-code-audit-20261003/DELETION_PLAN.md) 分前端孤立模块、Go 纯 helper、inspection 旧链三组。保留 `pages → features/components/entities/services/stores/hooks/utils` 方向和 `model → repository → service → controller → router → httpapi → cmd` 分层。

baseResolver 的仍适用解析语义落在当前 service-base 入口；删除只覆盖旧实现的测试，不保留第二套解析实现。SplashScreen 样式仅在独占消费者确认后随组件删除；共享品牌资源不动。Inspection 无入口操作链一起删，当前 WithContext 路径、错误传播与两个产品模式保持。xAI reset 继续使用通用当前 parser。

## Tradeoffs, compatibility and rollback

不新增替代组件、抽象、配置开关或兼容层。只按当前源码证明的冗余清理，出现生产消费者则报告并交由主代理裁决，不扩大接口退役。

source-only 无部署动作；构建只是验证单文件面板契约，不手改嵌入 HTML。按三组保留可独立验收的局部 diff；恢复只撤销任务拥有修改并保留并发内容，不覆盖整个文件。
