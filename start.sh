#!/usr/bin/env bash
set -e
cd "$(dirname "$0")"

if pgrep -f "./qoder2api --instance" > /dev/null; then
  echo "qoder2api 已经在运行中。"
  exit 0
fi

nohup ./qoder2api --instance=cn > app.log 2>&1 &
sleep 1

if pgrep -f "./qoder2api --instance" > /dev/null; then
  echo "qoder2api 启动成功！"
  echo "控制台访问地址: http://127.0.0.1:3588"
else
  echo "启动失败，请查看 app.log"
fi
