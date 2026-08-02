# syntax=docker/dockerfile:1

# moment-go 开发容器：
#   - Go 工具链（与 go.mod 的 go 1.26.1 对齐）
#   - Android NDK（与 libgphoto2-android 的构建环境一致：NDK r27d）
#   - 完整 NDK 环境变量 + PATH + cgo
#
# 构建参数可覆盖：
#   GO_VERSION / NDK_VERSION / ANDROID_API_LEVEL / GOPROXY
# 例如：
#   docker build -f .devcontainer/Dockerfile \
#     --build-arg GOPROXY=https://proxy.golang.com.cn,direct .

ARG GO_VERSION=1.26.1
FROM golang:${GO_VERSION}-bookworm


ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

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

# 强制开启 cgo 动态链接（libgphoto2 通过 cgo 绑定）
ENV CGO_ENABLED=1

RUN apt-get update \
    && apt-get install -y --no-install-recommends unzip \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fSL -o /tmp/android-ndk.zip \
        "https://dl.google.com/android/repository/android-ndk-${NDK_VERSION}-linux.zip" \
    && unzip -q /tmp/android-ndk.zip -d /opt \
    && mv "/opt/android-ndk-${NDK_VERSION}" "${ANDROID_NDK_HOME}" \
    && rm -f /tmp/android-ndk.zip

# 自检（构建镜像时即验证 cgo 与 NDK 可用）：
#   1. cgo 已正确开启（输出 enabled / 1）
#   2. Go 交叉编译工具链（从 386 弹出 386 值，android/amd64 宿主侧已解决）
#   3. NDK aarch64 clang 可用
RUN echo "go env CGO_ENABLED = $(go env CGO_ENABLED)" && \
    echo "go env GOPROXY   = $(go env GOPROXY)" && \
    go version && \
    "${NDK_TOOLCHAIN}/aarch64-linux-android${ANDROID_API_LEVEL}-clang" --version

# devcontainer 默认把仓库挂载到 /workspaces/<目录名>
WORKDIR /workspaces/moment-go
