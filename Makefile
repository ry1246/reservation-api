.PHONY: run test vet

include .env
export

run:
	go run .

test:
	go test ./...

vet:
	go vet ./...
