.PHONY: build run install tidy check release

build:
	go build -o bin/mak .

run:
	go run . $(ARGS)

install:
	go install .

tidy:
	go mod tidy

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test ./...
	go build ./...

release:
	@test -n "$(v)" || (echo "Usage: make release v=0.1.0"; exit 1)
	@echo "$(v)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$$' || (echo "Version must be semantic, for example 0.1.0"; exit 1)
	@test -z "$$(git status --porcelain)" || (echo "Working tree must be clean before release"; exit 1)
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse '@{upstream}')" || (echo "Push the current commit before release"; exit 1)
	$(MAKE) check
	git tag -a v$(v) -m "release v$(v)"
	git push origin v$(v)
