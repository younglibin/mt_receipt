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
## Docker

推荐使用 Docker Compose 启动，端口映射和目录挂载已经写在 `docker-compose.yml`：

```bash
docker compose up -d --build
```

启动后在本地浏览器访问：

```text
http://localhost:8080
```

停止服务：

```bash
docker compose down
```

也可以不用 Compose，手动打包镜像：

```bash
docker build -f dockerfile -t bx_mt_project .
```

手动启动 Web 页面：

```bash
docker run --rm \
  -p 8080:8080 \
  -v "$(pwd)/output:/app/output" \
  -v "$(pwd)/uploads:/app/uploads" \
  bx_mt_project
```

运行容器时，建议挂载以下目录：

- 输出目录挂载到 `/app/output`
- 上传目录挂载到 `/app/uploads`

可参考以下目录映射：

- 本地输出目录：接收程序生成结果
- 容器输出目录：`/app/output`
- 本地上传目录：保存页面上传的文件
- 容器上传目录：`/app/uploads`
