FROM golang:1.25-alpine@sha256:e6898559d553d81b245eb8eadafcb3ca38ef320a9e26674df59d4f07a4fd0b07 AS builder

WORKDIR /authserver
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/go/pkg \
  go build -o /bin/authserver ./cmd

FROM alpine:3.23.2@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62
COPY --from=builder /bin/authserver /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/authserver" ]
