.PHONY: generate build run verify

generate:
	cd ../.. && $(MAKE) gen

build:
	go build ./cmd/jobs

run:
	go run ./cmd/jobs --config config/defaults.json

verify:
	go build ./... && go vet ./... && test -z "$$(gofmt -l .)"
