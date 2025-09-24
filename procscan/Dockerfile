FROM golang:1.244-alpine AS builder

ARG TARGETARCH
COPY bin/service-complik-$TARGETARCH /manager

# 创建非 root 用户（ID 65532 对应 nonroot 用户）
RUN groupadd -g 65532 nonroot && \
    useradd -u 65532 -g 65532 -m -s /bin/bash nonroot

# 设置权限
RUN chmod +x /manager

EXPOSE 8428
USER 65532:65532

# 修正 ENTRYPOINT，移除固定的配置文件路径
ENTRYPOINT [ "/manager" ]


CMD ["./process-scanner"]