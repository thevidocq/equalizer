# syntax=docker/dockerfile:1
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/eqtool ./cmd/eqtool

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/eqtool /eqtool
WORKDIR /work
ENTRYPOINT ["/eqtool"]
