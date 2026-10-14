#!/usr/bin/env bash
cd "$(dirname "$0")"

if pkill -f "./qoder2api --instance"; then
  echo "qoder2api 进程已终止。"
else
  echo "未找到运行中的 qoder2api 进程。"
fi
