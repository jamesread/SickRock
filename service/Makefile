.PHONY: all deps build run test tidy clean

BIN := sickrock

all: build

deps:
	go mod tidy

build:
	go build -o ./tmp/$(BIN) ./...

run:
	go run ./main.go

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf ./tmp/$(BIN)


