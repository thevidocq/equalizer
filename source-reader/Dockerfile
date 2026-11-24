# syntax=docker/dockerfile:1
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/source-reader ./cmd/source-reader

# rsync over ssh for sftp, smbclient for smb, ssh-keygen for the reader's key.
FROM alpine:3.20
RUN apk add --no-cache openssh-client openssh-keygen rsync samba-client ca-certificates \
 && adduser -D -u 10001 reader \
 && mkdir -p /var/lib/equalizer-source /tmp/equalizer-source \
 && chown reader:reader /var/lib/equalizer-source /tmp/equalizer-source
COPY --from=build /out/source-reader /usr/local/bin/source-reader
USER reader
# No inbound port: the reader only calls the panel. The site's folder, for
# the local method, is mounted read only at /site.
VOLUME ["/var/lib/equalizer-source"]
ENTRYPOINT ["/usr/local/bin/source-reader"]
