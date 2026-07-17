# LibreChat MCP 绑定示例

## 1. librechat.yaml 配置

```yaml
mcpServers:
  bx-mt-project:
    type: streamable-http
    url: http://bx_mt_project_web:8080/mcp
    timeout: 60000
    serverInstructions: true
    chatMenu: true
    headers:
      X-User-ID: '{{LIBRECHAT_USER_ID}}'
```

## 2. Agent 说明词

可将下面内容放到 LibreChat Agent 的系统提示词中：

```text
你可以使用 bx-mt-project MCP 服务提供的四个工具：

1. receipt_calculate
- 用途：根据 MT 文件和 receipt 文件执行算账
- 必填参数：
  - mt_file_path
  - receipt_file_path

2. monthly_settlement
- 用途：根据 result.xlsx 执行月度算账
- 必填参数：
  - result_file_path

3. latest_receipt_summary
- 用途：查询最近一次 receipt 输出文件中的设计师汇总
- 可选参数：
  - output_dir

4. latest_employee_settlement
- 用途：输入员工名字，查询该员工最近一次结算信息
- 必填参数：
  - employee_name
- 可选参数：
  - output_dir

调用规则：
- 如果没有容器内可访问的文件路径，不要伪造上传或执行结果
- 只有在用户明确要求执行算账或月度算账时才调用工具
- 必须先确认文件路径是容器内可访问的共享路径
- 如果路径不存在，要直接告诉用户文件不可访问，而不是伪造执行结果
- 当用户要查看最近一次算账结果汇总时，优先调用 latest_receipt_summary
- 当用户要查看某个执行设计师最近一次结算明细时，优先调用 latest_employee_settlement
- 优先把工具返回的 structuredContent.message 和 output 整理后再反馈给用户
```

## 3. 工具入参约定

### receipt_calculate

```json
{
  "mt_file_path": "/app/uploads/MT.xlsx",
  "receipt_file_path": "/app/uploads/receipt.xls"
}
```

### monthly_settlement

```json
{
  "result_file_path": "/app/uploads/result.xlsx"
}
```

### latest_receipt_summary

```json
{}
```

如需显式指定输出目录：

```json
{
  "output_dir": "/app/output"
}
```

### latest_employee_settlement

```json
{
  "employee_name": "王天成"
}
```

如需显式指定输出目录：

```json
{
  "employee_name": "王天成",
  "output_dir": "/app/output"
}
```

## 4. MCP 调试请求示例

### initialize

```bash
curl -s http://localhost:8080/mcp \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}'
```

### tools/list

```bash
curl -s http://localhost:8080/mcp \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
```

### tools/call - receipt_calculate

```bash
curl -s http://localhost:8080/mcp \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"receipt_calculate","arguments":{"mt_file_path":"/app/uploads/MT.xlsx","receipt_file_path":"/app/uploads/receipt.xls"}}}'
```

### tools/call - monthly_settlement

```bash
curl -s http://localhost:8080/mcp \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"monthly_settlement","arguments":{"result_file_path":"/app/uploads/result.xlsx"}}}'
```

### tools/call - latest_receipt_summary

```bash
curl -s http://localhost:8080/mcp \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"latest_receipt_summary","arguments":{}}}'
```

### tools/call - latest_employee_settlement

```bash
curl -s http://localhost:8080/mcp \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"latest_employee_settlement","arguments":{"employee_name":"王天成"}}}'
```
