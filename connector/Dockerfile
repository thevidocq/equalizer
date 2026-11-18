# syntax=docker/dockerfile:1
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/connector ./cmd/connector

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/connector /connector
USER nonroot:nonroot
# No inbound port: the connector only opens the tunnel towards the relay.
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=3 CMD ["/connector", "healthcheck"]
ENTRYPOINT ["/connector"]
