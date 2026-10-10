FROM golang:1.27-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /server ./cmd/api
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /server ./server
COPY data ./data
USER app
EXPOSE 8080
ENTRYPOINT ["/app/server"]
