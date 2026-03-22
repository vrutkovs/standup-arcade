FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o meet-attendees . \
 && mkdir -p /data

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /data /data
COPY --from=builder /app/meet-attendees /meet-attendees
VOLUME ["/data"]
ENV TOKEN_CACHE_FILE=/data/.meet-attendees-token.json
EXPOSE 8080
ENTRYPOINT ["/meet-attendees", "0.0.0.0:8080"]
