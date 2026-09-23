# ---------- 构建阶段 ----------
FROM golang:1.24-alpine AS builder

WORKDIR /src

# 国内网络走 goproxy.cn 加速
ENV GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY controller ./controller
COPY core ./core
COPY public ./public

RUN go build -ldflags "-s -w -extldflags -static" -o /out/webssh .

# ---------- 运行阶段 ----------
FROM alpine:3.20

WORKDIR /webssh

RUN apk add --no-cache curl tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apk del tzdata

COPY --from=builder /out/webssh /webssh/webssh

EXPOSE 8888/tcp

ENV PORT=8888

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -sf http://127.0.0.1:${PORT}/check >/dev/null || exit 1

ENTRYPOINT ["/webssh/webssh"]
CMD ["-p", "8888"]
