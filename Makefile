# Linters are versioned in internal/tools/go.mod and run with "go tool", so
# the versions are pinned, they share compatible versions of dependencies
# (like golang.org/x/tools), and there's no chance of running some other
# installed binary of the same name.
GO_TOOL := go tool -modfile=internal/tools/go.mod

.PHONY: ci
ci: deps checkgofmt errcheck golint vet ineffassign staticcheck test

.PHONY: deps
deps:
	go get -d -v -t ./...

.PHONY: updatedeps
updatedeps:
	go get -d -v -t -u -f ./...

.PHONY: install
install:
	go install ./...

.PHONY: checkgofmt
checkgofmt:
	@echo gofmt -s -l .
	@output="$$(gofmt -s -l .)" ; \
	if [ -n "$$output"  ]; then \
	    echo "$$output"; \
		echo "Run gofmt on the above files!"; \
		exit 1; \
	fi

.PHONY: vet
vet:
	go vet ./...

.PHONY: staticcheck
staticcheck:
	$(GO_TOOL) staticcheck ./...

.PHONY: ineffassign
ineffassign:
	$(GO_TOOL) ineffassign .

.PHONY: golint
golint:
	$(GO_TOOL) golint -min_confidence 0.9 -set_exit_status ./...

.PHONY: errcheck
errcheck:
	$(GO_TOOL) errcheck ./...

.PHONY: test
test:
	go test -cover -race ./...

.PHONY: generate
generate:
	go generate ./...

.PHONY: testcover
testcover:
	@echo go test -race -covermode=atomic ./...
	@echo "mode: atomic" > coverage.out
	@for dir in $$(go list ./...); do \
		go test -race -coverprofile profile.out -covermode=atomic $$dir ; \
		if [ -f profile.out ]; then \
			tail -n +2 profile.out >> coverage.out && rm profile.out ; \
		fi \
	done

