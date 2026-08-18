# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod ./
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/pim ./cmd/pim

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 65532 pim
WORKDIR /app
COPY --from=build /out/pim /app/pim
RUN mkdir -p /data && chown -R pim:pim /data
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/pim"]
