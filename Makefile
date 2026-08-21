.PHONY: build test race vet fmt run-service run-tool docker-build docker-build-arm tidy

build:
	go build ./...

test:
	go test -timeout=300s -count=1 ./...

race:
	go test -race -timeout=420s -count=1 ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

run-service:
	go run ./cmd/civicbridge

run-tool:
	go run ./cmd/civicctl

docker-build:
	docker build --platform linux/amd64 -t civicbridge:linux-amd64 .

docker-build-arm:
	docker build --platform linux/arm64 -t civicbridge:linux-arm64 .

tidy:
	go mod tidy
