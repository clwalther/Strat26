FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/strat26 .

FROM alpine:3.22
RUN addgroup -S strat26 && adduser -S -G strat26 strat26
WORKDIR /app
COPY --from=build /out/strat26 ./strat26
COPY --from=build /src/source ./source
RUN mkdir -p /app/database && chown -R strat26:strat26 /app
USER strat26
ENV PORT=8080 DATABASE_PATH=/app/database/database.db
EXPOSE 8080
VOLUME ["/app/database"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O - http://127.0.0.1:8080/api/health || exit 1
ENTRYPOINT ["./strat26"]
