# TODO

本文档用于跟踪 `mymtr` 的实现任务与里程碑交付（以 `docs/` 中的架构/技术/API 设计为准）。

## 里程碑 0：仓库与基建

- [ ] 初始化 Go 工程骨架：`cmd/mymtr`、`internal/{mtr,geoip,cli,tui}`、`pkg/types`
- [ ] 补齐工程入口：`go.mod`、`Makefile`（`build/test/lint/run`）
- [ ] 统一错误与输出规范：参数校验、权限错误（raw socket）、运行时错误分层
- [ ] CI（可选）：`go test ./...`、`golangci-lint`、跨平台构建

## 里程碑 1：核心探测 MVP（CLI 单次输出）

- [ ] 目标解析：域名解析、IPv4/IPv6 选择、超时/重试策略
- [ ] `mtr.Config` 与 `mtr.Controller`：生命周期、事件/快照接口（先快照即可）
- [ ] `mtr.Prober` 接口落地：
  - [ ] ICMP Prober：raw socket 权限检查、Echo request/response 匹配（id/seq）、TTL 控制
  - [ ] UDP Prober：发送 UDP 并解析 ICMP（可先占位，后续补齐）
- [ ] Hop/Stats 最小实现：每 TTL 记录 IP/RTT/丢包与基础统计
- [ ] CLI：`mymtr <target>` + flags（`--max-hops --count --interval --timeout --protocol --ip-version --json`）
- [ ] 输出：文本表格（默认），JSON（可先占位）

验收：`mymtr example.com --count 10` 能输出每跳 RTT/丢包（需要 raw socket 权限时给出明确提示）。

## 里程碑 2：统计完善 + JSON 输出

- [x] `HopStats` 完整指标：sent/recv、loss%、last/best/avg/worst/stddev、history ring buffer
- [x] JSON schema 稳定：字段命名、duration/time 序列化策略（`schema_version` + `*_ms`）
- [x] `--json`：单次执行输出结构化结果

## 里程碑 3：GeoIP 解析

- [x] `geoip.GeoResolver` 接口与空实现（解析失败不报错）
- [x] 先接入 `cip.cc/<ip>`：HTTP 获取并解析（内存缓存 + TTL，避免调用过多被屏蔽）
- [x] ip2region 集成：xdb 加载/关闭（IPv4），通过 `--geoip ip2region --ip2region-db <path>` 启用
- [ ] 可选后端：GeoIP2、QQWry（按需）
- [x] 输出联动：CLI/JSON 输出 `location`（Country/Province/City/ISP + Source）

## 里程碑 4：TUI 实时模式

- [x] Bubbletea 模型：事件驱动刷新、按 TTL 更新、状态栏（目标/协议/轮次）
- [x] 交互：暂停/继续、退出（切换显示后续按需补）
- [x] 性能：并发安全（channel/锁），事件队列满时丢弃避免阻塞

## 里程碑 5：健壮性、测试与发布

- [ ] 权限与跨平台：非 root 提示、capabilities 指南、不同 OS 限制说明
- [ ] 单元测试：stats、序列化、解析逻辑；必要的接口 mock
- [ ] E2E（可选）：受限环境用例或跳过 raw socket 的 CI 策略
- [ ] 文档：`README`（安装/示例/权限/GeoIP 数据来源）与 `docs/` 同步
- [ ] 发布：交叉编译、压缩包、变更日志
