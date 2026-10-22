.PHONY: all deps build run test lint fmt tidy clean gocyclo

BIN := sickrock

all: build

deps:
	go mod tidy

build:
	go build -o ./tmp/$(BIN)

run:
	go run .

test:
	go test ./...

GOCYCLO_IGNORE := '(^|/)gen(/|$)'

lint:
	go vet ./...
	@unformatted=$$(gofmt -l $$(find . -name '*.go' -not -path './gen/*')); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed (run make fmt):"; echo "$$unformatted"; exit 1; fi

gocyclo:
	gocyclo -ignore $(GOCYCLO_IGNORE) .

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './gen/*')

tidy:
	go mod tidy

clean:
	rm -rf ./tmp/$(BIN)

prep:
	go mod download
