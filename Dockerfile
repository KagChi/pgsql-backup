FROM golang:1.21-alpine as build-stage

WORKDIR /tmp/build

COPY . .

# Build the project
RUN go build .

FROM alpine:3

LABEL name "PGSQL Auto Backup"
LABEL maintainer "KagChi"

WORKDIR /app

# Install needed deps
RUN apk add --no-cache tzdata tini postgresql-client

COPY --from=build-stage /tmp/build/pgsql-backup main

ENTRYPOINT ["tini", "--"]
CMD ["/app/main"]