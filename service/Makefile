.PHONY: all deps build run test lint tidy clean

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

lint:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf ./tmp/$(BIN)

prep:
	go mod download
