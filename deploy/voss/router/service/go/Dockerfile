FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./...
RUN CGO_ENABLED=0 go build -o router .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/router .
EXPOSE 8080
ENV VOSS_CONFIG=/config/tenants.production.json
ENTRYPOINT ["./router"]