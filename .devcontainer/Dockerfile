# syntax=docker/dockerfile:1

# moment-go 开发容器：
#   - Go 工具链（与 go.mod 的 go 1.26.1 对齐）
#   - Android NDK（与 libgphoto2-android 的构建环境一致：NDK r27d）
#   - 完整 NDK 环境变量与 PATH
#
# 构建参数可覆盖：
#   GO_VERSION / NDK_VERSION / ANDROID_API_LEVEL
# 例如：
#   docker build -f .devcontainer/Dockerfile \
#     --build-arg NDK_VERSION=r26d --build-arg ANDROID_API_LEVEL=21 .

ARG GO_VERSION=1.26.1
FROM golang:${GO_VERSION}-bookworm

ARG NDK_VERSION=r27d
ARG ANDROID_API_LEVEL=27

# ---- NDK 相关环境变量（容器内全局生效，postCreate / 构建脚本均可直接使用）----
ENV ANDROID_NDK_HOME=/opt/android-ndk
ENV ANDROID_NDK_ROOT=${ANDROID_NDK_HOME}
ENV ANDROID_HOME=/opt/android-sdk
ENV ANDROID_API_LEVEL=${ANDROID_API_LEVEL}
# NDK clang 所在目录（linux-x86_64 = linux/amd64 容器宿主）
ENV NDK_TOOLCHAIN=${ANDROID_NDK_HOME}/toolchains/llvm/prebuilt/linux-x86_64/bin
ENV PATH=${NDK_TOOLCHAIN}:${PATH}

# golang 基础镜像自带 git / curl / gcc / make / ca-certificates；
# 仅需补充 unzip 用于解压 NDK。
RUN apt-get update \
    && apt-get install -y --no-install-recommends unzip \
    && rm -rf /var/lib/apt/lists/*

# 下载并解压 Android NDK。
# 注：NDK 压缩包无需接受 license，解压即用。
RUN curl -fSL -o /tmp/android-ndk.zip \
        "https://dl.google.com/android/repository/android-ndk-${NDK_VERSION}-linux.zip" \
    && unzip -q /tmp/android-ndk.zip -d /opt \
    && mv "/opt/android-ndk-${NDK_VERSION}" "${ANDROID_NDK_HOME}" \
    && rm -f /tmp/android-ndk.zip

# 交叉编译工具链自检（构建镜像时即验证 NDK 可用）
RUN "${NDK_TOOLCHAIN}/aarch64-linux-android${ANDROID_API_LEVEL}-clang" --version \
    && go version

# devcontainer 默认把仓库挂载到 /workspaces/<目录名>
WORKDIR /workspaces/moment-go
