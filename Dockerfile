FROM golang:1.26-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/anime-backend ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache \
    ca-certificates \
    chromium \
    harfbuzz \
    nss \
    ttf-freefont \
 && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=build /out/anime-backend /usr/local/bin/anime-backend

USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/anime-backend"]
