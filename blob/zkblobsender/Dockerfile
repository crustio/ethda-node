# CONTAINER FOR BUILDING BINARY
FROM golang:1.21 AS build

# INSTALL DEPENDENCIES
COPY go.mod go.sum /src/
RUN cd /src && go mod download

# BUILD BINARY
COPY . /src
RUN cd /src && make build-zkblob

# CONTAINER FOR RUNNING BINARY
FROM alpine:3.18
COPY --from=build /src/dist/zkblob-sender /app/zkblob-sender
RUN apk update && apk add postgresql15-client
CMD ["/bin/sh", "-c", "/app/zkblob-sender run"]
