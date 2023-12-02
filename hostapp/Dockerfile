# Stage 1 - Build the Go binary
FROM golang:1.21.3 as builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o hostapp .

# Stage 2 - Create a minimal Docker image for the binary
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY insert_spb.sql.gohtml .
COPY --from=builder /app/hostapp .
CMD ["./hostapp"]