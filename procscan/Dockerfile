FROM golang:1.24.5 AS builder
ARG TARGETARCH
COPY bin/service-procscan-$TARGETARCH /manager
EXPOSE 8428
USER 65532:65532
ENTRYPOINT [ "/manager", "config/config.yml" ]