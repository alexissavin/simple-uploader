FROM golang:1.21 AS build-env
LABEL org.opencontainers.image.authors="Alexis Savin"

RUN mkdir -p /go/src/app
WORKDIR /go/src/app

# resolve dependency before copying whole source code
COPY go.mod .
COPY go.sum .
RUN go mod download

# copy other sources & build
COPY . /go/src/app
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /go/bin/app
RUN mkdir -p /etc/simple_uploader && mkdir -p /var/html/simple_uploader/data

FROM istio/distroless:latest AS runtime-env

LABEL org.opencontainers.image.authors="Alexis Savin"

COPY --from=build-env /go/bin/app /usr/local/bin/app

COPY --from=build-env /etc/simple_uploader /etc/simple_uploader
COPY --from=build-env /var/html/simple_uploader/data /var/html/simple_uploader/data

EXPOSE 8080/tcp
ENTRYPOINT ["/usr/local/bin/app"]

# test build: docker build -t testglake .
# vuln test: 