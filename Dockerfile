# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27

# ---- Build stage ----
# Run the compiler natively on the build machine and let Go cross-compile
# for the target platform (much faster than emulating arm64 under QEMU).
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

WORKDIR /src

# Download dependencies first so this layer stays cached until go.mod/go.sum change.
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Populated automatically by buildx from --platform (e.g. linux/arm64).
ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/server .

# ---- Runtime stage ----
# distroless/static: no shell or package manager, but includes CA certs,
# tzdata, and a non-root user (uid 65532).
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /server

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/server"]
