# BX_MT_Project

## 环境要求

- Go 版本：`1.21`
- 默认输入目录：[File](file:///Users/bytedance/GolandProjects/BX_MT_Project/File)
- 默认输出目录：[output](file:///Users/bytedance/GolandProjects/BX_MT_Project/output)
- 设计师配置文件：[config/designers.json](file:///Users/bytedance/GolandProjects/BX_MT_Project/config/designers.json)

本地执行时，建议使用下面的环境变量：

```bash
export PATH=/opt/homebrew/Cellar/go@1.21/1.21.13_1/bin:$PATH
export GOROOT=/opt/homebrew/Cellar/go@1.21/1.21.13_1/libexec
export GOSUMDB=off
```

## 命令说明

项目现在使用两个子命令：

- `receipt`：原有双文件处理逻辑
- `settlement`：新增结算拆分逻辑

查看帮助：

```bash
go run . receipt -h
go run . settlement -h
```

## receipt 命令

用途：

- 读取 `receipt` 和 `MT` 两个文件
- 生成结果文件和回写后的 `receipt` 文件

默认读取：

- [File/receipt.xlsx](file:///Users/bytedance/GolandProjects/BX_MT_Project/File/receipt.xlsx) 或 [File/receipt.xls](file:///Users/bytedance/GolandProjects/BX_MT_Project/File/receipt.xls)
- [File/MT.xlsx](file:///Users/bytedance/GolandProjects/BX_MT_Project/File/MT.xlsx) 或 [File/MT.xls](file:///Users/bytedance/GolandProjects/BX_MT_Project/File/MT.xls)

默认输出：

- [output](file:///Users/bytedance/GolandProjects/BX_MT_Project/output)`/yyyyMMdd_HHmmss_result.xlsx`
- [output](file:///Users/bytedance/GolandProjects/BX_MT_Project/output)`/yyyyMMdd_HHmmss_receipt_filled.xlsx`

执行示例：

```bash
cd /Users/bytedance/GolandProjects/BX_MT_Project && \
GOROOT=/opt/homebrew/Cellar/go@1.21/1.21.13_1/libexec \
PATH=/opt/homebrew/Cellar/go@1.21/1.21.13_1/bin:$PATH \
GOSUMDB=off \
go run . receipt
```

如需手动指定文件：

```bash
cd /Users/bytedance/GolandProjects/BX_MT_Project && \
GOROOT=/opt/homebrew/Cellar/go@1.21/1.21.13_1/libexec \
PATH=/opt/homebrew/Cellar/go@1.21/1.21.13_1/bin:$PATH \
GOSUMDB=off \
go run . receipt \
  -receipt /path/to/receipt.xls \
  -mt /path/to/MT.xlsx \
  -out /path/to/result.xlsx \
  -receipt-out /path/to/receipt_filled.xlsx
```

### receipt 新增逻辑

- 会读取 [config/designers.json](file:///Users/bytedance/GolandProjects/BX_MT_Project/config/designers.json) 中的 `records`
- 根据 `result` 表中的“执行设计师”匹配 `name`
- 自动补充两列：
  - `结算公司`
  - `设计类型`

## settlement 命令

用途：

- 输入一个 `xlsx` 文件
- 基于第一个工作表，按“结算公司”拆分成多个 sheet

默认读取：

- [File/result.xlsx](file:///Users/bytedance/GolandProjects/BX_MT_Project/File/result.xlsx)

默认输出：

- [output](file:///Users/bytedance/GolandProjects/BX_MT_Project/output)`/yyyyMMdd_HHmmss_settlement.xlsx`

执行示例：

```bash
cd /Users/bytedance/GolandProjects/BX_MT_Project && \
GOROOT=/opt/homebrew/Cellar/go@1.21/1.21.13_1/libexec \
PATH=/opt/homebrew/Cellar/go@1.21/1.21.13_1/bin:$PATH \
GOSUMDB=off \
go run . settlement
```

如需手动指定输入输出文件：

```bash
cd /Users/bytedance/GolandProjects/BX_MT_Project && \
GOROOT=/opt/homebrew/Cellar/go@1.21/1.21.13_1/libexec \
PATH=/opt/homebrew/Cellar/go@1.21/1.21.13_1/bin:$PATH \
GOSUMDB=off \
go run . settlement \
  -input /path/to/input.xlsx \
  -out /path/to/output.xlsx
```

### settlement 处理规则

- 只读取第一个工作表
- 按 `结算公司` 拆分 sheet
- 每个结算公司生成一个 sheet
- 每个 sheet 只保留以下 10 列：
  - `需求方事业部`
  - `需求ID`
  - `PO单号`
  - `需求名称`
  - `设计类型`
  - `执行设计师`
  - `结算公司金额`
  - `实际金额`
  - `开票日期`
  - `结算公司`
- 其中：
  - `结算公司金额` 保留空值
  - `开票日期` 保留空值
  - 其他 8 列从源 sheet 对应列名取值
- 同一个结算公司内：
  - 相同设计师的数据连续放置
  - 不同设计师之间空两行

## designers.json 配置

配置文件路径：

- [config/designers.json](file:///Users/bytedance/GolandProjects/BX_MT_Project/config/designers.json)

结构说明：

```json
{
  "fieldMapping": {
    "name": "执行设计师名字",
    "designType": "设计类型",
    "settlementCompany": "结算公司"
  },
  "records": [
    {
      "name": "王雨萌",
      "designType": "包装",
      "settlementCompany": "万象更芯"
    }
  ]
}
```

匹配规则：

- `receipt` 逻辑中，执行设计师单元格只按单个名字精确匹配
- 如果在 `records` 中找不到对应名字，则 `结算公司` 和 `设计类型` 留空

## Docker

打包镜像：

```bash
docker build -t bx_mt_project .
```

运行容器时，建议挂载两个目录：

- 输入目录挂载到 `/app/File`
- 输出目录挂载到 `/app/output`

可参考以下目录映射：

- 本地输入目录：存放 `MT.xlsx`、`receipt.xls`、`result.xlsx`
- 容器输入目录：`/app/File`
- 本地输出目录：接收程序生成结果
- 容器输出目录：`/app/output`
