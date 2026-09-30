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
# Release version from cocogitto (CI passes it); shown in the page footer.
ARG VERSION=dev
ENV LDFLAGS="-X github.com/jeffwnelson/sodatracker/internal/buildinfo.Version=${VERSION}"
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "$LDFLAGS" -o /out/server ./cmd/server
# The rpi tag selects the periph.io GPIO provider instead of the mock.
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "$LDFLAGS" -tags rpi -o /out/piagent ./cmd/piagent

# --- Build React frontend ---
FROM --platform=$BUILDPLATFORM node:22-alpine AS web-build
WORKDIR /src
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# --- Fetch tini for the target arch without running target-arch code ---
# The final stage has no RUN steps, so building arm64 needs no QEMU; the
# self-hosted runners' Talos kernel has no binfmt_misc to emulate with.
FROM --platform=$BUILDPLATFORM alpine:3.21 AS tini
ARG TARGETARCH
RUN case "$TARGETARCH" in \
      amd64) arch=x86_64 ;; \
      arm64) arch=aarch64 ;; \
      *) echo "unsupported TARGETARCH $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && apk add --no-cache --no-scripts --initdb --root /rootfs --arch "$arch" \
      --keys-dir "/usr/share/apk/keys/$arch" \
      --repositories-file /etc/apk/repositories \
      tini-static

# --- Final image ---
# ca-certificates-bundle and busybox wget (used by the entrypoint's health
# wait) are already in the alpine base image.
FROM alpine:3.21
# Links the GHCR package to this repo, which is what lets the repo's Actions
# token push to it.
LABEL org.opencontainers.image.source="https://github.com/jeffwnelson/sodatracker"
LABEL org.opencontainers.image.description="SodaTracker API, dashboard, and GPIO pi agent"
LABEL org.opencontainers.image.licenses="MIT"
COPY --from=tini /rootfs/sbin/tini-static /sbin/tini
WORKDIR /app
COPY --from=api-build /out/server /out/piagent ./
COPY --from=web-build /src/dist ./static
COPY --chmod=755 docker-entrypoint.sh /usr/local/bin/entrypoint
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
