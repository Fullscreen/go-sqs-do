GOOS ?= linux
GOARCH ?= amd64
VERSION ?= $(shell awk '/Version/ {gsub("\"", "", $$4); print $$4}' version.go)

XC_OS ?= darwin linux
XC_ARCH ?= 386 amd64

GOBIN := $(GOPATH)/bin
DEBFILE := stdiolog_$(VERSION)-0_$(GOARCH).deb

all: test

deps:
	go get -d -t ./...

test: deps
	go test ./...

install: deps
	go install

# Create a github release
release:
	@gox -os="$(XC_OS)" -arch="$(XC_ARCH)" -output "dist/{{.OS}}_{{.Arch}}/{{.Dir}}"
	@for platform in $$(find ./dist -mindepth 1 -maxdepth 1 -type d); do \
		pushd $$platform >/dev/null; \
		zip ../$$(basename $$platform).zip ./* >/dev/null; \
		popd >/dev/null; \
	done
	@ghr -u fullscreen $(VERSION) dist/

clean:
	@rm -rf dist/

.PHONY: clean release test

