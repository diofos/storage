build:
	@go build -o bin/fs

run: build
	@./bin/fs

build-meshstore:
	@go build -o bin/meshstore ./cmd/meshstore

run-meshstore: build-meshstore
	@./bin/meshstore

build-diofos:
	@go build -o bin/diofos ./cmd/meshstore

run-diofos: build-diofos
	@./bin/diofos

dist:
	@mkdir -p bin/dist
	@echo "Building DIOFOS zero-install cross-platform binaries..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/dist/diofos-darwin-arm64 ./cmd/meshstore
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/dist/diofos-darwin-amd64 ./cmd/meshstore
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/dist/diofos-linux-amd64 ./cmd/meshstore
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/dist/diofos-linux-arm64 ./cmd/meshstore
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/dist/diofos-windows-amd64.exe ./cmd/meshstore
	@echo "All pre-compiled binaries created in bin/dist/"

test:
	@go test ./...
