BINARIES := $(notdir $(shell find cmd -mindepth 1 -maxdepth 1 -type d))

.PHONY: $(BINARIES)
.PHONY: all
.PHONY: build
.PHONY: test
.PHONY: cleantest
.PHONY: vet
.PHONY: staticcheck
.PHONY: lint
.PHONY: clean
.PHONY: install-deps

all: vet lint staticcheck test build
build: $(BINARIES)

# When building binaries 
$(BINARIES):
	@echo "*** building $@"
	@cd cmd/$@ && CGO_ENABLED=0 go build -trimpath -o ../../bin/$@

test:
	@echo "*** $@"
	@go test ./...

cleantest:
	@echo "*** $@"
	@go clean -testcache

vet:
	@echo "*** $@"
	@go vet ./...

staticcheck:
	@echo "*** $@"
	@staticcheck ./...

lint:
	@echo "*** $@"
	@revive ./...

clean:
	@echo "*** cleaning binaries"
	@rm -rf bin

install-deps:
	@go install github.com/mgechev/revive@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
