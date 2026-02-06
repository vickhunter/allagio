FROM golang:1.23-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -o /bin/affilio ./cmd/server

FROM alpine:3.19
COPY --from=build /bin/affilio /bin/affilio
EXPOSE 8080
CMD ["/bin/affilio"]
