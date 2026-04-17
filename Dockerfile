FROM golang:1.25 AS builder

WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/simple_ai ./main.go

FROM alpine:3.22

WORKDIR /app

RUN set -eux; \
    ALPINE_VERSION="$(cut -d. -f1,2 /etc/alpine-release)"; \
    MIRROR="https://mirrors.tencent.com/alpine"; \
    printf '%s\n%s\n' \
      "${MIRROR}/v${ALPINE_VERSION}/main" \
      "${MIRROR}/v${ALPINE_VERSION}/community" \
      > /etc/apk/repositories; \
    INSTALLED=0; \
    for i in 1 2 3; do \
      if apk add --no-cache ca-certificates tzdata; then \
        INSTALLED=1; \
        break; \
      fi; \
      echo "apk add failed with tencent mirror, retry ${i}/3"; \
      sleep 3; \
    done; \
    if [ "${INSTALLED}" -ne 1 ]; then \
      echo "apk add failed with tencent mirror"; \
      exit 1; \
    fi

COPY --from=builder /out/simple_ai /app/simple_ai
COPY config /app/config

EXPOSE 9090

CMD ["/app/simple_ai"]
