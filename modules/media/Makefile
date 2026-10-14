.PHONY: generate build run verify

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/media

run:
	go run ./cmd/media --config config/service.defaults.json

verify:
	go build ./...
	go vet ./...
	test -z "$$(gofmt -l .)"
