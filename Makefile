.PHONY: build run install tidy

build:
	go build -o bin/mak .

run:
	go run . $(ARGS)

install:
	go install .

tidy:
	go mod tidy
