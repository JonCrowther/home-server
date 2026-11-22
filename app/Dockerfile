# Built by Home Assistant on the Pi itself, so unlike the main Dockerfile it
# needs no cross-compiling or BuildKit-only features.

ARG GO_VERSION=1.27

FROM golang:${GO_VERSION} AS build

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server .

# Not the :nonroot variant: Home Assistant's /data/options.json is only
# readable by root.
FROM gcr.io/distroless/static-debian12

COPY --from=build /out/server /server

ENTRYPOINT ["/server"]
