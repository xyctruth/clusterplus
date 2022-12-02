FROM golang:1.18  as builder
WORKDIR /workspace
COPY ./ ./


RUN GOPROXY="https://goproxy.cn,direct" CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on \
    go build -o web ./main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/web .

EXPOSE 8080

ENTRYPOINT ["/web"]
