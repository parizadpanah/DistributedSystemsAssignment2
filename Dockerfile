FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/kvstore ./main.go

FROM scratch
WORKDIR /app
COPY --from=builder /out/kvstore /app/kvstore
ENV APP_ADDR=:8080
ENV DATA_DIR=/app/data
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app/kvstore"]
