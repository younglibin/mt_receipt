# 使用官方的Go 1.21镜像作为基础
FROM golang:1.21-alpine

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制所有源代码
COPY . .

# 编译项目
RUN go build -o bx_mt_project .

# 创建必要的目录
RUN mkdir -p File output

# 设置默认命令
CMD ["./bx_mt_project"]
