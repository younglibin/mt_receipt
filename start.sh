#!/bin/sh
set -eu

mkdir -p output uploads

docker compose up -d --build

echo "服务已启动，请在浏览器访问: http://localhost:8080"
