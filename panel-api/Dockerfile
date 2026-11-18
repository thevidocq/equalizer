# syntax=docker/dockerfile:1
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/panel-api ./cmd/panel-api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/panel-api /panel-api
USER nonroot:nonroot
EXPOSE 8080 9092
HEALTHCHECK --interval=15s --timeout=5s --retries=3 CMD ["/panel-api", "healthcheck"]
ENTRYPOINT ["/panel-api"]
