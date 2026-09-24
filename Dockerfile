# --- Build Go binaries (cross-compiled from the build host, no emulation) ---
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS api-build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
ENV CGO_ENABLED=0
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/server ./cmd/server
# The rpi tag selects the periph.io GPIO provider instead of the mock.
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags rpi -o /out/piagent ./cmd/piagent

# --- Build React frontend ---
FROM --platform=$BUILDPLATFORM node:22-alpine AS web-build
WORKDIR /src
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# --- Final image ---
FROM alpine:3.21
# Links the GHCR package to this repo, which is what lets the repo's Actions
# token push to it.
LABEL org.opencontainers.image.source="https://github.com/jeffwnelson/sodatracker"
LABEL org.opencontainers.image.description="SodaTracker API, dashboard, and GPIO pi agent"
LABEL org.opencontainers.image.licenses="MIT"
RUN apk add --no-cache ca-certificates tini
WORKDIR /app
COPY --from=api-build /out/server /out/piagent ./
COPY --from=web-build /src/dist ./static
COPY docker-entrypoint.sh /usr/local/bin/entrypoint
RUN chmod +x /usr/local/bin/entrypoint
ENV PORT=3000
ENV STATIC_DIR=/app/static
ENV DB_PATH=/data/sodatracker.db
ENV LOG_FORMAT=json
ENV PIAGENT_CONFIG=/config/piagent.yaml
ENV PIAGENT_DB_PATH=/data/piagent-cache.db
EXPOSE 3000
VOLUME /data
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/entrypoint"]
CMD ["all"]
