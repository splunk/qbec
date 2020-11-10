include Makefile.tools

VERSION         := 0.12.5
SHORT_COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
GO_VERSION      := $(shell go version | awk '{ print $$3}' | sed 's/^go//')

LEARN_THEME_TAG := 2.2.0
# When modifying this, also modify the corresponding ldflags in .goreleaser.yaml
LD_FLAGS_PKG ?= github.com/splunk/qbec/internal/commands
LD_FLAGS :=
LD_FLAGS +=  -X "$(LD_FLAGS_PKG).version=$(VERSION)"
LD_FLAGS +=  -X "$(LD_FLAGS_PKG).commit=$(SHORT_COMMIT)"
LD_FLAGS +=  -X "$(LD_FLAGS_PKG).goVersion=$(GO_VERSION)"

LINT_FLAGS ?=
TEST_FLAGS ?=

export GO111MODULE=on
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.DEFAULT_GOAL := all

.PHONY: all
all: build lint test

.PHONY: get
get:
	go get ./...

.PHONY: build
build:
	go install -ldflags '$(LD_FLAGS)' ./...

.PHONY: test
test:
	go test $(TEST_FLAGS) -coverprofile=coverage.txt -covermode=atomic -race ./...

.PHONY: publish-coverage
publish-coverage:
	# publishes results from coverage.txt
	curl -s https://codecov.io/bash | bash

.PHONY: lint
lint: check-format
	go vet ./...
	golint ./...
	golangci-lint run $(LINT_FLAGS) .

.PHONY: check-format
check-format:
	@echo "Running gofmt..."
	$(eval unformatted=$(shell find . -name '*.go' | grep -v ./.git | grep -v vendor | xargs gofmt -s -l))
	$(if $(strip $(unformatted)),\
		$(error $(\n) Some files are ill formatted! Run: \
			$(foreach file,$(unformatted),$(\n)    gofmt -s -w $(file))$(\n)),\
		@echo All files are well formatted.\
	)


.PHONY: install-ci
install-ci: HELM_VERSION := 3.3.1
install-ci: HELM_PLATFORM := $(shell uname|  tr '[:upper:]' '[:lower:]')
install-ci:
	# Refactor helm install into a separate step
	# curl -sSL -o helm.tar.gz https://get.helm.sh/helm-v${HELM_VERSION}-${HELM_PLATFORM}-amd64.tar.gz
	# tar -xvzf helm.tar.gz
	# mv ${HELM_PLATFORM}-amd64/helm $(GOPATH)/bin/
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin v1.21.0

.PHONY: create-cluster
create-cluster:	.tools/kind
	.tools/kind create cluster

.PHONY: install
install:
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
	@echo build O/S archive for: $(GOOS)-$(GOARCH)
	rm -rf dist/tmp
	mkdir -p dist/tmp
	mkdir -p dist/assets
ifeq ($(GOOS), windows)
	go build -ldflags '$(LD_FLAGS)' -o dist/tmp/qbec.exe .
	go build -ldflags '$(LD_FLAGS)' -o dist/tmp/jsonnet-qbec.exe ./cmd/jsonnet-qbec
	(cd dist/tmp && zip ../assets/qbec-$(GOOS)-$(GOARCH).zip *)
else
	go build -ldflags '$(LD_FLAGS)' -o dist/tmp/qbec .
	go build -ldflags '$(LD_FLAGS)' -o dist/tmp/jsonnet-qbec ./cmd/jsonnet-qbec
	(cd dist/tmp && tar -czf ../assets/qbec-$(GOOS)-$(GOARCH).tar.gz *)
endif
	rm -rf dist/tmp

.PHONY: release-notes
release-notes:
	go test cmd/changelog-extractor/*.go
	go run cmd/changelog-extractor/main.go CHANGELOG.md > .release-notes.md
