REPO=malice
NAME=engine
VERSION=$(shell cat .release/VERSION)
MESSAGE?="New release"

# Packages to test (everything except vendor).
SOURCE_FILES?=$$(go list ./... | grep -v /vendor/)
TEST_PATTERN?=.
TEST_OPTIONS?=

GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo none)
GIT_DIRTY=$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GIT_DESCRIBE=$(git describe --tags --always 2>/dev/null || echo none)

.PHONY: bindata build test fmt lint ci help

bindata: ## Embed binary data (config + plugins toml) into the binary
	@echo "===> Embedding Binary Data"
	rm -f config/bindata.go plugins/bindata.go
	go-bindata -pkg config -ignore=load.go config/...
	mv bindata.go config/bindata.go
	go-bindata -pkg plugins -ignore="^.*.go|\.DS_Store" plugins/...
	mv bindata.go plugins/bindata.go

build: bindata ## Build the malice binary (malice-bin)
	@echo "===> Building $(NAME) ($(VERSION))"
	go build -buildvcs=false -ldflags "-X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o malice-bin .

test: ## Run all the tests
	@echo "===> Running Tests"
	go test $(TEST_OPTIONS) -cover $(SOURCE_FILES)

fmt: ## gofmt all go files
	@echo "===> Formatting Go Files"
	find . -name "*.go" -not -path "./vendor/*" -not -name "bindata.go" -print0 | xargs -0 gofmt -w -s

lint: ## Static checks (go vet + gofmt)
	@echo "===> Linting"
	go vet ./...
	@bad=$$(gofmt -l . | grep -v bindata.go); if [ -n "$$bad" ]; then echo "gofmt needed on:"; echo "$$bad"; exit 1; fi

ci: lint test ## Run all the tests and code checks

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
