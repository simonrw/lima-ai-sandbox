BINARY := sandbox
MODULE := github.com/simonrw/lima-ai-sandbox

.PHONY: build install test clean

build:
	go build -o $(BINARY) .

install:
	go install .

test:
	go test ./...

clean:
	rm -f $(BINARY)
