FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -o /bin/affilio ./cmd/server

FROM alpine:3.19
COPY --from=build /bin/affilio /bin/affilio
EXPOSE 8080
CMD ["/bin/affilio"]
