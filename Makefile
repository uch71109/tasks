.PHONY: run generate test lint docker-build docker-run

# Keep in sync with the golangci-lint version in .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION := v2.14.0

run:
	go run ./main

generate:
	go generate ./...

test:
	go test -race ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

docker-build:
	docker build -t tasks-api .

docker-run: docker-build
	docker run --rm -p 8080:8080 tasks-api
