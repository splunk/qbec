VERSION         := 0.6.2
SHORT_COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
GO_VERSION      := $(shell go version | awk '{ print $$3}' | sed 's/^go//')

LEARN_THEME_TAG := 2.2.0

LD_FLAGS_PKG ?= main
LD_FLAGS :=
LD_FLAGS +=  -X "$(LD_FLAGS_PKG).version=$(VERSION)"
LD_FLAGS +=  -X "$(LD_FLAGS_PKG).commit=$(SHORT_COMMIT)"
LD_FLAGS +=  -X "$(LD_FLAGS_PKG).goVersion=$(GO_VERSION)"

.PHONY: all
all: get build lint test

.PHONY: get
get:
	dep ensure

.PHONY: build
build:
	go install -ldflags '$(LD_FLAGS)' ./...

.PHONY: test
test:
	go test -v ./...

.PHONY: lint
lint:
	go list ./... | grep -v vendor | xargs go vet
	go list ./... | grep -v vendor | xargs golint

.PHONY: install-ci
install-ci:
	curl -sSL -o helm.tar.gz https://storage.googleapis.com/kubernetes-helm/helm-v2.13.1-linux-amd64.tar.gz
	tar -xvzf helm.tar.gz
	mv linux-amd64/helm $(GOPATH)/bin/

.PHONY: install
install:
	go get github.com/golang/dep/cmd/dep
	go get golang.org/x/lint/golint
	@echo for building docs, manually install hugo for your OS from: https://github.com/gohugoio/hugo/releases

.PHONY: site
site:
	cd site && rm -rf themes/
	mkdir -p site/themes
	git clone https://github.com/matcornic/hugo-theme-learn site/themes/learn
	(cd site/themes/learn && git checkout -q $(LEARN_THEME_TAG) && rm -rf exampleSite  && rm -f images/* && rm -f CHANGELOG.md netlify.toml wercker.yaml .grenrc.yml)
	cd site && hugo

.PHONY: clean
clean:
	rm -rf vendor/
	rm -rf site/themes
	rm -rf site/public

.PHONY: os_archive
os_archive:
	rm -rf dist/tmp
	mkdir -p dist/tmp
ifeq ($(GOOS), windows)
	go build -ldflags '$(LD_FLAGS)' -o dist/tmp/qbec.exe .
	go build -ldflags '$(LD_FLAGS)' -o dist/tmp/jsonnet-qbec.exe ./cmd/jsonnet-qbec
	(cd dist/tmp && zip ../assets/qbec-${GOOS}-${GOARCH}.zip *)
else
	go build -ldflags '$(LD_FLAGS)' -o dist/tmp/qbec .
	go build -ldflags '$(LD_FLAGS)' -o dist/tmp/jsonnet-qbec ./cmd/jsonnet-qbec
	(cd dist/tmp && tar -czf ../assets/qbec-${GOOS}-${GOARCH}.tar.gz *)
endif
	rm -rf dist/tmp
