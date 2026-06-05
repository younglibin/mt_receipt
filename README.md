# BX_MT_Project

## 环境要求

- Go 版本：`1.21`

## 命令说明

项目现在使用两个子命令：

- `receipt`：原有双文件处理逻辑
- `settlement`：新增结算拆分逻辑
- `web`：启动本地页面，通过浏览器选择文件并执行算账逻辑

查看帮助：

```bash
go run . receipt -h
go run . settlement -h
go run . web -h
```

## 本地页面

启动页面：

```bash
go run . web
```

打开浏览器访问：

```text
http://localhost:8080
```

页面包含两组功能：

- 第一组“算账”：选择 `MT` 文件和 `receipt` 文件，点击“执行”后等价执行 `receipt -mt <MT文件路径> -receipt <receipt文件路径>`
- 第二组“月度算账”：选择 `result` 文件，点击“月度算账”后等价执行 `settlement -input <result文件路径>`

说明：

- 浏览器无法直接读取用户电脑上的真实本地文件路径，页面会先把文件上传到本地 Go 服务的 `uploads` 目录
- 服务端会把上传后的本地文件路径传给 `receipt` 或 `settlement` 命令执行
- 输出文件仍按原逻辑生成到 `output` 目录

## receipt 命令

用途：

- 读取 `receipt` 和 `MT` 两个文件
- 生成结果文件和回写后的 `receipt` 文件

执行示例：

```bash
go run . receipt
```

如需手动指定文件：

```bash
go run . receipt \
  -receipt /path/to/receipt.xls \
  -mt /path/to/MT.xlsx \
  -out /path/to/result.xlsx \
  -receipt-out /path/to/receipt_filled.xlsx
```


## settlement 命令

用途：

- 输入一个 `xlsx` 文件
- 基于第一个工作表，按“结算公司”拆分成多个 sheet



执行示例：

```bash

go run . settlement
```

如需手动指定输入输出文件：

```bash
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
