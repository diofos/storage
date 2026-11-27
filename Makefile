build:
	@go build -o bin/fs

run: build
	@./bin/fs

build-meshstore:
	@go build -o bin/meshstore ./cmd/meshstore

run-meshstore: build-meshstore
	@./bin/meshstore

test:
	@go test ./...
