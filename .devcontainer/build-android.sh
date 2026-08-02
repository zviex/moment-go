#!/usr/bin/env bash
# 交叉编译 Android (arm64) 目标。
#
# 依赖容器/环境内已配置（见 .devcontainer/Dockerfile 的 ENV）：
#   ANDROID_NDK_HOME / ANDROID_API_LEVEL
#
# 用法：
#   bash .devcontainer/build-android.sh [输出路径，默认 bin/app-android-arm64]
set -euo pipefail

NDK="${ANDROID_NDK_HOME:-/opt/android-ndk}"
API="${ANDROID_API_LEVEL:-27}"
CC="${NDK}/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android${API}-clang"
OUT="${1:-bin/app-android-arm64}"

if [ ! -x "${CC}" ]; then
  echo "error: NDK clang not found: ${CC}" >&2
  echo "hint: 检查 ANDROID_NDK_HOME / ANDROID_API_LEVEL 是否正确" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

mkdir -p "$(dirname "${OUT}")"
GOOS=android GOARCH=arm64 CGO_ENABLED=1 CC="${CC}" \
  go build -o "${OUT}" ./app

echo "built: ${OUT}"
