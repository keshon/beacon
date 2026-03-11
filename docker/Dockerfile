FROM golang:1.25.7-alpine3.22 AS mybuildstage

# Set Go proxy
ENV GOPROXY=https://proxy.golang.org,direct

WORKDIR /usr/project

# 1. Copy only go.mod and go.sum first to cache dependencies
COPY src/go.mod src/go.sum ./

# 2. Download modules (cached)
RUN go mod download

# 3. Copy the rest of the source code
COPY src .

# 4. Build the binary
RUN go build -o app ./cmd/uptime/main.go

# ---- Runtime stage ----
FROM alpine:3.22

WORKDIR /usr/project

# Copy binary and assets/templates if needed
COPY --from=mybuildstage /usr/project/app /usr/project/app
COPY --from=mybuildstage /usr/project/templates /usr/project/templates

ENTRYPOINT ["/usr/project/app"]
