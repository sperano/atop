# syntax=docker/dockerfile:1

# The vigie image: `vigie serve` with the built web UI embedded (-tags
# webdist), for linux/amd64 and linux/arm64. The web and go stages run on
# the build platform and Go cross-compiles for the target, so the UI is
# built once and neither Node nor Go runs under emulation: no QEMU needed.
#
#   docker buildx build --platform linux/amd64,linux/arm64 .

# Node satisfying web/package.json's engines, the major CI uses.
FROM --platform=$BUILDPLATFORM node:22.23-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json web/.npmrc ./
# No dependency needs an install script (the browser for the tests is
# downloaded by an explicit command, never at install), so none runs.
RUN --mount=type=cache,target=/root/.npm npm ci --ignore-scripts
COPY web/ ./
# Writes dist/, and dist/.vite/license.md for the bundled dependencies.
RUN npm run build

# The module's go line.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS go
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
# No fallback: without web/dist, the webdist build fails to compile.
COPY --from=web /src/web/dist web/dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
	go build -tags webdist -trimpath -ldflags "-s -w" -o /out/vigie .
# The image distributes the binary, so it carries the licenses of what it
# links.
RUN --mount=type=cache,target=/go/pkg/mod GOFLAGS=-tags=webdist sh scripts/collect-licenses.sh /out/licenses
# The hidden-rows file's directory, for a volume to be mounted on.
RUN mkdir /out/data

# scratch: a static binary needs no libc, shell or user database.
FROM scratch
# GitHub and Vikunja are reached over public HTTPS; in the cluster, client-go
# uses the service account's CA for the API server.
COPY --from=go /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=go /src/LICENSE /licenses/vigie/LICENSE
COPY --from=go /out/licenses /licenses
COPY --from=web /src/web/dist/.vite/license.md /licenses/web/license.md
COPY --from=go /out/vigie /vigie
# The server keeps its hidden rows here (scratch has no home directory for
# the default path). Mount a writable volume on /data to keep them across
# restarts; the directory is the runtime user's for a writable root too.
COPY --from=go --chown=65532:65532 /out/data /data
ENV VIGIE_HIDDEN_FILE=/data/hidden.json
# Numeric, so that Kubernetes' runAsNonRoot can check it without a passwd
# file.
USER 65532:65532
# config.DefaultListen; --listen or VIGIE_LISTEN changes it.
EXPOSE 8080
# No HEALTHCHECK: scratch has no shell or curl; probe GET /healthz (alive)
# and GET /readyz (first refresh done) instead.
# Exec form, so SIGTERM reaches the binary, which shuts down gracefully.
ENTRYPOINT ["/vigie", "serve"]
