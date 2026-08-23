FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api

FROM alpine:3.21
RUN adduser -D -H app
USER app
COPY --from=build /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
