FROM golang:1.27.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server && CGO_ENABLED=0 go build -trimpath -o /out/runner ./cmd/runner && CGO_ENABLED=0 go build -trimpath -o /out/hash-password ./cmd/hash-password

FROM alpine:3.23 AS api
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 nexcode && mkdir /data /logs && chown nexcode /data /logs
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/hash-password /usr/local/bin/hash-password
USER nexcode
EXPOSE 8080
ENTRYPOINT ["server"]

FROM docker:29-cli AS runner
COPY --from=build /out/runner /usr/local/bin/runner
EXPOSE 8090
ENTRYPOINT ["runner"]
