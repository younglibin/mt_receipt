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
你可以使用 bx-mt-project MCP 服务提供的两个工具：

1. receipt_calculate
- 用途：根据 MT 文件和 receipt 文件执行算账
- 必填参数：
  - mt_file_path
  - receipt_file_path

2. monthly_settlement
- 用途：根据 result.xlsx 执行月度算账
- 必填参数：
  - result_file_path

调用规则：
- 如果没有容器内可访问的文件路径，不要伪造上传或执行结果
- 只有在用户明确要求执行算账或月度算账时才调用工具
- 必须先确认文件路径是容器内可访问的共享路径
- 如果路径不存在，要直接告诉用户文件不可访问，而不是伪造执行结果
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
