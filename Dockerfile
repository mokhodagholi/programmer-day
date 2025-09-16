# -------- Builder stage --------
FROM golang:1.22 AS builder
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# -------- Runtime stage --------
FROM alpine:latest

# Set the working directory
WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/main .
COPY --from=builder /app/assets/questions.json ./assets/questions.json
COPY --from=builder /app/assets/users.json ./assets/users.json
COPY --from=builder /app/assets/state.json ./assets/state.json

# Expose the port your application listens on (if applicable)
EXPOSE 8080

# Command to run the application
CMD ["./main"]

