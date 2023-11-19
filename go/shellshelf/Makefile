ifeq (, $(shell which golangci-lint))
$(warning "could not find golangci-lint in $(PATH), get it at https://golangci-lint.run/usage/install/")
endif

.PHONY: build clean fmt lint run test install uninstall

default: all

all: fmt lint test

build:
	$(info ============================================================> building)
	go build main.go
clean:
	$(info ============================================================> cleaning)
	rm -f main
fmt:
	$(info ============================================================> formatting)
	go fmt ./...
lint:
	$(info ============================================================> linting)
	golangci-lint run -v
run:
	$(info ============================================================> running)
	go run main.go
test:
	$(info ============================================================> testing)
	go test -v ./...

install:
	$(info ============================================================> installing)
	go build -o ss main.go && mv ss $(GOPATH)
uninstall:
	$(info ============================================================> uninstalling)
	rm -f $(GOPATH)/ss
