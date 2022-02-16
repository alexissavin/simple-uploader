FROM golang:1.16 AS build-env
MAINTAINER Alexis Savin

RUN mkdir -p /go/src/app
WORKDIR /go/src/app

# resolve dependency before copying whole source code
COPY go.mod .
COPY go.sum .
RUN go mod download

# copy other sources & build
COPY . /go/src/app
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /go/bin/app

FROM debian:bullseye-slim AS runtime-env
MAINTAINER Alexis Savin
ARG DEBIAN_FRONTEND=noninteractive

RUN groupadd -g 999 goapp
RUN adduser --uid 999 --ingroup goapp --system --disabled-password --disabled-login --no-create-home goapp

RUN mkdir -p /etc/simple_uploader/tokens
RUN chown -R goapp:goapp /etc/simple_uploader/tokens
RUN chmod -R 750 /etc/simple_uploader/tokens
RUN mkdir -p /var/html/simple_uploader/data
RUN chown -R goapp:goapp /var/html/simple_uploader/data
RUN chmod -R 770 /var/html/simple_uploader

RUN ls -ld /etc/simple_uploader/tokens
RUN ls -ld /var/html/simple_uploader/data

COPY ./entrypoint.sh /usr/local/bin/entrypoint.sh
COPY --from=build-env /go/bin/app /usr/local/bin/app

EXPOSE 8080/tcp
ENTRYPOINT ["/bin/sh", "/usr/local/bin/entrypoint.sh"]