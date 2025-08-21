FROM golang:1.24.6-alpine AS builder

# Set working directory inside container
WORKDIR /app

# Install git if needed
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the app
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server .

FROM alpine:latest

WORKDIR /root/

# Copy the compiled binary from builder stage
COPY --from=builder /app/server .


EXPOSE 8080

# Command to run the binary
CMD ["./server"]
