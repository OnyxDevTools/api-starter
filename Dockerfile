# Dockerfile
FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /api ./cmd/api

FROM alpine:3.24.3
RUN apk add --no-cache ca-certificates && addgroup -g 10001 app && adduser -D -u 10001 -G app app
WORKDIR /app
COPY --from=build /api /app/api
USER 10001:10001
ENV API_HOST=0.0.0.0 API_PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/api"]
