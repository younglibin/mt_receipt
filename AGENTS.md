# AGENTS.md

## 1. 项目概览

这是一个面向 Excel 文件的本地算账工具，采用 **Go 单体程序 + `service/` 业务层 + 可选 Web 页面 + Docker 部署** 的结构。

- 主入口在 `main.go:18`，通过子命令分流为：
  - `receipt`：读取 `receipt` 与 `MT` 两个文件，按 PO 匹配并生成结果文件，同时回写执行设计师到 receipt。见 `main.go:37`。
  - `settlement`：读取 `result.xlsx`，按“结算公司”拆分多个 Sheet。见 `main.go:143`。
  - `web`：提供本地上传页面，上传后复用当前可执行文件执行已有子命令。见 `main.go:179`、`main.go:287`。
- 业务逻辑基本集中在 `service/`：
  - Excel 读取与通用辅助：`service/common.go:21`
  - receipt 汇总与 MT 匹配：`service/calc.go:54`、`service/calc.go:79`
  - result 输出：`service/result.go:10`
  - receipt 回写与设计师汇总：`service/receipt.go:13`、`service/receipt.go:104`
  - settlement 拆分：`service/settlement.go:31`
  - 表头校验：`service/validator.go:20`
  - 设计师配置加载：`service/designer_config.go:20`
- 当前没有独立前端工程；Web 页面直接以内嵌 HTML 常量实现，见 `main.go:325`。仓库中的 `web/` 目录不是运行入口。
- 现有文档里没有发现 `AGENT.md` / `AGENTS.md` 历史文件，也没有发现 Cursor / Copilot / Trae 规则文件。

### 核心数据流

1. `receipt` 模式先校验模板表头，再汇总 receipt 的未付款金额，随后在 MT 中按 PO 匹配，输出 `result.xlsx`。见 `main.go:81`、`main.go:105`、`main.go:113`。
2. 结果文件会追加“差值 / 结算公司 / 设计类型”三列，后两者来自 `config/designers.json`。见 `service/result.go:17`、`service/result.go:89`。
3. `receipt_filled.xlsx` 会写回执行设计师，并额外生成“设计师汇总”Sheet。见 `main.go:131`、`service/receipt.go:152`。
4. `settlement` 模式要求输入文件包含指定业务列，然后按“结算公司”拆 Sheet、按执行设计师分组。见 `service/settlement.go:51`、`service/settlement.go:117`。

## 2. 构建、运行与常用命令

### Go 命令

- 查看帮助：
  - `go run . receipt -h`
  - `go run . settlement -h`
  - `go run . web -h`
  - 来源：`README.md:17`
- 本地启动 Web：
  - `go run . web`
  - 默认地址：`http://localhost:8080`
  - 来源：`README.md:27`
- 执行 receipt：
  - `go run . receipt`
  - 或显式传参：
    `go run . receipt -receipt /path/to/receipt.xls -mt /path/to/MT.xlsx -out /path/to/result.xlsx -receipt-out /path/to/receipt_filled.xlsx`
  - 来源：`README.md:57`
- 执行 settlement：
  - `go run . settlement`
  - 或显式传参：
    `go run . settlement -input /path/to/input.xlsx -out /path/to/output.xlsx`
  - 来源：`README.md:83`

### Docker / 脚本

- Docker Compose 启动：`docker compose up -d --build`，见 `README.md:99`、`docker-compose.yml:1`
- Docker Compose 停止：`docker compose down`，见 `README.md:111`
- 手动构建镜像：`docker build -f dockerfile -t bx_mt_project .`，见 `README.md:118`
- 一键脚本：
  - `./start.sh`：先创建 `output/`、`uploads/` 再启动 Compose，见 `start.sh:4`
  - `./stop.sh`：停止 Compose，见 `stop.sh:4`

### 测试 / 验证

- 当前仓库 **没有任何 `_test.go` 文件**。
- `go test ./...` 目前只起到“编译级验证”作用；我在仓库上执行后结果为：
  - `BX_MT_Project [no test files]`
  - `BX_MT_Project/service [no test files]`
- 若本机 Go 版本与 `GOROOT` 指向不一致，测试会先因环境错配失败；仓库要求 Go `1.21`，见 `go.mod:3`、`README.md:5`。
- 实际业务验证主要依赖以下方式：
  - 使用模板与真实 Excel 文件手工执行 `receipt` / `settlement`
  - 启动 `web` 后通过浏览器上传文件进行端到端验证

## 3. 代码风格与修改约定

仅记录仓库中已经体现出来的约定，不补充通用 Go 教条。

- 入口编排写在 `main.go`，细节逻辑下沉到 `service/`；新增业务优先放到 `service/`，不要继续膨胀 `main.go`。现有结构见 `main.go:37`、`main.go:143`、`main.go:179`。
- 与 Excel 列相关的固定位置使用常量，不在业务代码里散落硬编码列号。现有列常量集中在 `service/consts.go:3`。
- 读取表格时统一走 `LoadRowsGeneric`，由它决定 `.xls` 还是 `.xlsx`。见 `service/common.go:21`。
- 业务函数倾向于“返回 error，由上层命令统一 `log.Fatalf` 退出”。例如 `service/validator.go:20` 返回错误，`main.go:89`、`main.go:99` 负责终止流程。
- 排序和衍生字段写入是业务语义的一部分，改动时不要只看字段，还要保留输出顺序：
  - `result.xlsx` 先按 E 列对接设计师第一个名字，再按 G 列执行设计师排序，见 `service/result.go:34`
  - `receipt_filled.xlsx` 先按 L 列品类经理，再按 P 列执行设计师排序，见 `service/receipt.go:68`
  - `settlement` 每个结算公司 Sheet 内按执行设计师分组，组间空两行，见 `service/settlement.go:125`
- 命名上以中文业务字段为核心，尤其是表头、Sheet 名、错误信息；新增字段时优先沿用已有中文命名，不要擅自英文化。参考 `service/settlement.go:13`、`service/result.go:18`。
- 仓库未发现额外 lint / format 配置；默认按 Go 工具链风格维护代码即可。未发现 `golangci-lint`、`staticcheck` 或自定义格式化规则。

## 4. 测试说明

### 当前现状

- 无单元测试、无集成测试、无测试数据夹具目录。
- `service/` 下大部分逻辑是可测试的纯函数或低副作用函数，尤其适合优先补测：
  - `parseMoney`：`service/calc.go:22`
  - `SumUnpaidByPO`：`service/calc.go:55`
  - `FilterMTRowsByPO`：`service/calc.go:80`
  - `buildHeaderIndex` / `uniqueSheetName`：`service/settlement.go:109`、`service/settlement.go:174`
  - `BuildDesignerRecordMap` / `FindDesignerRecord`：`service/designer_config.go:33`、`service/designer_config.go:45`

### 执行建议

- 基础验证先跑：`go test ./...`
- 涉及 Excel 输出的改动，至少补一轮真实文件回归：
  1. 用模板或样例文件执行 `receipt`
  2. 检查 `output/` 中结果文件是否生成
  3. 再把结果文件输入 `settlement`
  4. 检查 Sheet 命名、分组、追加列和排序是否符合预期
- 修改 Web 上传逻辑后，需额外验证：
  - `POST /api/receipt`，见 `main.go:206`
  - `POST /api/settlement`，见 `main.go:235`

## 5. 安全与数据保护

本仓库的安全重点不是鉴权框架，而是 **本地文件上传、Excel 数据落盘、业务敏感配置**。

- Web 服务默认对外监听 `:8080`，且没有认证、鉴权或权限控制。见 `main.go:181`、`main.go:186`、`docker-compose.yml:8`。
- 上传文件会保存到 `uploads/`，结果文件会写到 `output/`；这两个目录可能包含真实业务数据，不应直接提交到公共仓库，也不应长期无清理保留。写入点见 `main.go:266`、`service/result.go:111`、`service/settlement.go:106`。
- 上传时只保留扩展名并加时间戳重命名，能避免直接用用户文件名覆盖，但 **没有校验 MIME / 真实文件内容**。见 `main.go:269`。
- Web 模式通过 `exec.Command(exePath, args...)` 复用当前可执行文件，不经过 shell；不要把它改成 shell 拼接字符串。见 `main.go:297`。
- `config/designers.json` 存放了真实姓名、设计类型、结算公司映射，属于业务敏感配置；修改时避免泄露或将测试值混入生产映射。见 `config/designers.json:1`。
- `receipt` 依赖模板表头强校验，`settlement` 依赖必需列校验；新增输入字段时，必须同步更新校验规则，否则 Web 和 CLI 都可能接受错误文件或拒绝合法文件。见 `service/validator.go:20`、`service/settlement.go:51`。

## 6. 配置与环境

### 运行环境

- Go 版本：`1.21`，见 `go.mod:3`
- 核心依赖：
  - `github.com/xuri/excelize/v2`：处理 `.xlsx`，见 `go.mod:7`
  - `github.com/extrame/xls`：处理 `.xls`，见 `go.mod:6`

### 关键目录

- `config/`：运行时设计师映射配置；当前核心文件是 `config/designers.json`
- `template/`：输入模板，用于表头校验；`receipt` 和 `MT` 都依赖它
- `File/`：命令行默认输入目录；未传参时会从这里兜底读取，见 `main.go:62`、`main.go:156`
- `uploads/`：Web 上传暂存目录
- `output/`：程序输出目录

### Docker 配置

- 镜像是多阶段构建，运行阶段仅复制二进制、`config/`、`template/`，说明这两个目录是运行必需资源。见 `dockerfile:24`。
- 容器默认命令是 `./bx_mt_project web -addr :8080`，见 `dockerfile:35`。
- Compose 仅挂载 `output/` 和 `uploads/`；如果修改运行时所需目录，记得同步更新 `docker-compose.yml:10`。

## 7. 对后续开发最有价值的注意点

- 如果改动了 Excel 表头、列顺序或输出字段，优先检查：
  - `service/consts.go:3`
  - `service/validator.go:20`
  - `service/result.go:17`
  - `service/settlement.go:13`
- 如果改动了上传或 Web 逻辑，务必同时检查：
  - `saveUploadedFile` 的落盘规则：`main.go:259`
  - API 返回结构：`main.go:306`
  - 子命令复用是否仍成立：`main.go:287`
- 如果新增设计师字段或调整结算逻辑，优先从 `config/designers.json` 和 `service/designer_config.go:20` 入手，而不是把映射散落到代码里。
- 如果要补测试，优先测 `service/` 中的纯逻辑，再考虑文件级回归；这比直接从 `main.go` 切入更稳。


## 项目和Agent部署关系
- 调用层级 LibreChat（Agent）--> mt_receipt(MCP Server) -->BX_MT_Project(服务)
- 改项目是部署在docker容器中的， 我的agent liberchat 也是部署在docker 中，后续开发都会考虑在docker 中部署，访问都集中在docker中， 不会出现从docker调用我本地服务的情况
