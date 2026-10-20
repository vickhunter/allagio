FROM golang:1.23 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
# GIT_COMMIT is the build-time SHA. RENDER_GIT_COMMIT and
# RAILWAY_GIT_COMMIT_SHA still win at runtime when the host sets them.
ARG GIT_COMMIT=unknown
RUN go build -ldflags "-X affilio-api/internal/version.commit=${GIT_COMMIT}" -o /bin/affilio ./cmd/server

FROM ubuntu:24.04
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /bin/affilio /bin/affilio
EXPOSE 8080
CMD ["/bin/affilio"]
