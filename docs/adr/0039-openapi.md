# D39. 前后端接口以 OpenAPI 定义为准

- 状态：✅ 已定
- 日期：2026-10-12

- 接口写在 `api/openapi.yaml` 里，这是唯一依据。
- Go 端用 oapi-codegen 生成代码，目标是标准库 `net/http`；前端用 openapi-typescript 生成类型和请求客户端。
- 修改接口的流程：先改 `openapi.yaml`，再运行代码生成，然后两边按编译错误修改。
- `make check` 会检查生成的代码是否最新。
