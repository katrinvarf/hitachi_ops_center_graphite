FROM golang:alpine as builder

RUN apk --no-cache add git
RUN go get -u github.com/katrinvarf/hitachi_ops_center_graphite

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /

COPY --from=builder /go/bin/hitachi_ops_center_graphite ./usr/bin/hitachi_ops_center_graphite
CMD ["hitachi_ops_center_graphite", "-config", "/etc/config.yml", "-resource", "/etc/metrics.yml"]
