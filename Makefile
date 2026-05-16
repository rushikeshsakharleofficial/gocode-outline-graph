BINARY := code-outline-graph
TAGS   := fts5
BUILD  := go build -buildvcs=false -tags "$(TAGS)"

.PHONY: build install clean test

build:
	$(BUILD) -o $(BINARY) ./cmd/code-outline-graph/

install:
	$(BUILD) -o $(GOPATH)/bin/$(BINARY) ./cmd/code-outline-graph/

clean:
	rm -f $(BINARY)

test:
	go test -tags "$(TAGS)" ./...
