SHELL := /bin/bash
GO_FILES?=$(find . -name '*.go' | grep -v vendor)

# To provide the version use 'make release VERSION=1.1.1 GPGKEY=<example@efficientip.com>'
ifdef VERSION
	RELEASE := $(VERSION)
else
	RELEASE := 99999.9
endif

default: build

build:
	go get -v ./...
	go mod tidy
	go mod vendor
	env CGO_ENABLED=0 go build -o simple_uploader

fmt:
	gofmt -s -w ./*.go
	gofmt -s -w ./*.go

vet:
	go vet -all ./*.go