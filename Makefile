.PHONY: build run install tidy release

build:
	go build -o bin/mak .

run:
	go run . $(ARGS)

install:
	go install .

tidy:
	go mod tidy

release:
	@test -n "$(v)" || (echo "Usage: make release v=0.0.2"; exit 1)
	git add -A
	git commit -m "release v$(v)"
	git push origin main
	git tag v$(v)
	git push origin v$(v)
