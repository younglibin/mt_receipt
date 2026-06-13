# 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制所有源代码
COPY . .

# 编译项目
RUN CGO_ENABLED=0 GOOS=linux go build -o bx_mt_project .

# 运行阶段
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/bx_mt_project .
COPY --from=builder /app/config ./config
COPY --from=builder /app/template ./template

# 创建必要的目录
RUN mkdir -p File output uploads

# 暴露 Web 页面端口
EXPOSE 8080

# 默认启动 Web 页面
CMD ["./bx_mt_project", "web", "-addr", ":8080"]
