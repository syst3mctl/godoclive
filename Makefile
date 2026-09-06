.PHONY: ci

# Match the Go checks in .github/workflows/ci.yml. Node and jq exercise the
# generated JavaScript startup and the published composite Action shell steps.
ci:
	command -v node
	command -v jq
	go mod verify
	go build ./cmd/godoclive
	go test -race -count=1 ./...
	go vet ./...
	golangci-lint run
