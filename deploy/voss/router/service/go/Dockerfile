FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod tidy
COPY . .
RUN go build -o router .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/router .
EXPOSE 8080
CMD ["./router"]
