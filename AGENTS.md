# Repository Guidelines

## 项目结构与模块组织

当前仓库以设计文档为主，位于 `docs/`（如 `docs/architecture.md`、`docs/api-design.md`、`docs/technical-design.md`）。文档中描述了未来可能的 Go 工程目录（如 `cmd/`、`internal/`、`pkg/`、`data/`），但这些目录未必已落地；新增代码时请先与现有设计保持一致，再按需调整文档。

## 构建、测试与本地开发

当前未提供可执行代码与构建脚本。若后续引入 Go 实现，推荐的常用命令示例：
- 运行：`go run ./cmd/mymtr --help`
- 构建：`go build ./...`
- 测试：`go test ./...`

如新增 `Makefile`，请在本文档同步列出 `make build` / `make test` / `make lint` 等入口。

## 代码风格与命名约定

- 文档：使用 Markdown 标准语法，标题层级清晰，代码块标注语言（如 ```go）。
- 代码（若引入 Go/Odin 等）：优先遵循各语言社区惯例；目录与包名用小写、短横线或下划线按项目约定统一。
- 兼容性约束：编辑 `sdp` 文件中的结构体时，为保障兼容性不要修改任何 `tag`。

## 测试指南

当前无测试框架与覆盖率要求；若引入测试，建议采用同语言主流测试框架，并统一命名（如 Go：`*_test.go`），在 PR 中说明覆盖的场景与边界条件。

## 提交与 PR 规范

本目录当前未包含 `.git` 历史，无法归纳既有提交规范；建议采用 Conventional Commits（例如 `feat: ...`、`fix: ...`）。PR 至少应包含：变更说明、关联需求/问题、关键设计取舍；涉及 UI/输出格式时附截图或示例输出。

## Agent/自动化注意事项

- 使用 `lark-mcp` 访问飞书 API 时始终使用用户身份（`useUAT: true`）。
- 需要对 `https://moonton.feishu.cn/wiki` 的文档读写时，必须通过 `lark-mcp` 进行，不要直接抓取该站点 URL 内容。
- Odin 项目中获取 `x-biz-id` header 时，使用 `util/bind.CustomBind` 自动化获取，避免重复代码。
