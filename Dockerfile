# syntax=docker/dockerfile:1
FROM golang:1.26.8-alpine AS build
WORKDIR /src
COPY adapter-and-helper/go.mod adapter-and-helper/go.sum ./
RUN go mod download
COPY adapter-and-helper/ ./
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/adapter ./cmd/adapter

FROM build AS helper-linux-build
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/helper-linux ./cmd/helper

FROM scratch AS helper-linux-artifacts
COPY --from=helper-linux-build /out/helper-linux /helper-linux

FROM build AS helper-build
RUN CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/libhelper.so ./cmd/helper

FROM scratch AS helper-artifacts
COPY --from=helper-build /out/libhelper.so /arm64-v8a/libhelper.so

FROM alpine:3.23 AS adapter
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN addgroup -g 10001 app && adduser -D -H -u 10001 -G app app
COPY --from=build /out/adapter /usr/local/bin/adapter
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/adapter"]
CMD ["/config/adapter.config.yaml"]
