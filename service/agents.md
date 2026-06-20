# service/ agents.md

## 目录职责

`service/` 承载项目的核心业务逻辑，主要负责 Excel 文件读取、校验、汇总、匹配、结果输出、receipt 回写、settlement 拆分，以及设计师配置映射。

入口编排位于根目录 `main.go`，本目录应优先保持为可复用的业务层；新增业务逻辑时，优先放在本目录内的独立函数中，再由 `main.go` 调用。

## 关键文件

- `common.go`：Excel 读取与通用辅助，统一处理 `.xls` / `.xlsx`。
- `consts.go`：Excel 列号、固定列位置等常量定义。
- `calc.go`：receipt 未付款金额汇总、MT 按 PO 匹配等计算逻辑。
- `result.go`：生成 `result.xlsx`，追加差值、结算公司、设计类型等字段。
- `receipt.go`：回写 receipt 的执行设计师，并生成设计师汇总 Sheet。
- `settlement.go`：按结算公司拆分 Sheet，并按执行设计师分组输出。
- `validator.go`：模板表头校验。
- `designer_config.go`：加载并查询设计师配置映射。

## 修改约定

1. 与 Excel 列相关的固定位置应优先放到 `consts.go`，避免在业务代码中散落硬编码列号。
2. 新增或调整业务字段时，需同步检查表头校验、输出列顺序、排序逻辑和 settlement 拆分逻辑。
3. 业务函数应优先返回 `error`，由上层入口统一处理退出和日志输出。
4. 命名以现有中文业务字段为准，例如“执行设计师”“结算公司”“设计类型”，不要擅自英文化。
5. 保持输出排序和分组语义稳定：结果文件、receipt 回写文件和 settlement 输出都依赖业务排序规则。
6. 设计师、设计类型、结算公司等映射应来自 `config/designers.json`，不要将生产映射硬编码进业务逻辑。

## 输出文件排序规则

### result 输出

`result.go` 生成 `result.xlsx` 时，会先把原始行和对应差值绑定后再排序，避免排序后差值与原始行错位。

- 第一排序键：E 列“对接设计师”的第一个名字。
- 第二排序键：G 列“执行设计师”。
- 排序后再写入原始列、差值、结算公司、设计类型。
- 如果修改 E/G 列含义、列位置或设计师拆分规则，必须同步检查 `WriteResultXLSX` 中的排序逻辑。

### receipt 输出

`receipt.go` 生成 `receipt_filled.xlsx` 时，会保留标题行，将数据行补齐 P 列“执行设计师”后再排序写回。

- 第一排序键：L 列“品类经理”。
- 第二排序键：P 列“执行设计师”。
- P 列执行设计师来自 PO 到设计师的匹配结果，会覆盖写入到输出行中。
- 如果修改 L/P 列含义、列位置或 PO 到设计师的匹配逻辑，必须同步检查 `FillReceiptDesignerFromRows` 中的排序与写回逻辑。

## 验证建议

- 基础验证：`go test ./...`
- 涉及 Excel 输出的变更，建议用真实或模板文件手工跑通：
  1. `receipt` 生成 `result.xlsx` 和 `receipt_filled.xlsx`
  2. 使用 `result.xlsx` 执行 `settlement`
  3. 检查 Sheet 命名、追加列、排序、分组和金额差值是否符合预期

## 注意事项

- `uploads/`、`output/`、真实 Excel 文件和 `config/designers.json` 可能包含业务敏感信息，避免将测试数据或真实数据误提交。
- 修改 Web 上传相关逻辑时，虽然入口在 `main.go`，但仍要确认本目录中的校验、读取、输出函数是否兼容。
- 当前仓库缺少 `_test.go`，如后续补测试，优先覆盖本目录中副作用较小的函数，例如金额解析、PO 汇总、表头索引、Sheet 命名、设计师配置查询等。
