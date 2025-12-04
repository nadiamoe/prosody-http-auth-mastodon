FROM golang:1.25-alpine@sha256:26111811bc967321e7b6f852e914d14bede324cd1accb7f81811929a6a57fea9 AS builder

WORKDIR /authserver
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/go/pkg \
  go build -o /bin/authserver ./cmd

FROM alpine:3.23.0@sha256:51183f2cfa6320055da30872f211093f9ff1d3cf06f39a0bdb212314c5dc7375
COPY --from=builder /bin/authserver /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/authserver" ]
