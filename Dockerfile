# --- Build Go API ---
FROM golang:1.25-alpine AS api-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# --- Build React frontend ---
FROM node:22-alpine AS web-build
WORKDIR /src
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# --- Final image ---
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=api-build /server .
COPY --from=web-build /src/dist ./static
ENV PORT=3000
ENV STATIC_DIR=/app/static
ENV DB_PATH=/data/sodatracker.db
ENV LOG_FORMAT=json
EXPOSE 3000
VOLUME /data
CMD ["./server"]
