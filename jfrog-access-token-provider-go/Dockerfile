# Use official Golang image as a build stage
FROM golang:1.23 AS builder

# Set the working directory inside the container
WORKDIR /app

# -------------------------------------------------------------------
# ToDo - remove this workaround once the new GO SDK will be published
# Copy the local dependency folder
COPY secrets-manager-go-sdk/ ./secrets-manager-go-sdk/
# -------------------------------------------------------------------

# Copy go.mod and go.sum to leverage Docker caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Ensure the binary is built statically (better compatibility)
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/main.go

# Use a minimal image for the final container
FROM alpine:latest

# Set the working directory in the container
WORKDIR /root/

# Copy the built binary from the builder stage
COPY --from=builder /app/main .

# Run the binary
CMD ["./main"]
