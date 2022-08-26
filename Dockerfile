FROM golang:alpine AS builder

WORKDIR /src
COPY . /src

RUN go env -w GOPROXY=https://goproxy.cn,direct &&  \
    go build -o ping_exporter main.go

FROM alpine

WORKDIR /app
COPY --from=builder /src/ping_exporter /app/ping_exporter
COPY --from=builder /src/config.yaml /app/config.yaml
EXPOSE 9001

CMD ["/app/ping_exporter"]