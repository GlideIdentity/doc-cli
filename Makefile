GOFLAGS = -ldflags="-s -w"
GOPROXY_FLAGS = GONOSUMDB='*' GONOSUMCHECK='*'

.PHONY: build build-all clean

build:
	go build -o gdrive-bench ./cmd/gdrive-bench/
	go build -o gcs-bench ./cmd/gcs-bench/

build-all: clean
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o dist/gcs-bench-darwin-arm64 ./cmd/gcs-bench/
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -o dist/gcs-bench-darwin-amd64 ./cmd/gcs-bench/
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build $(GOFLAGS) -o dist/gcs-bench-linux-amd64  ./cmd/gcs-bench/
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build $(GOFLAGS) -o dist/gcs-bench-linux-arm64  ./cmd/gcs-bench/
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o dist/gdrive-bench-darwin-arm64 ./cmd/gdrive-bench/
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -o dist/gdrive-bench-darwin-amd64 ./cmd/gdrive-bench/
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build $(GOFLAGS) -o dist/gdrive-bench-linux-amd64  ./cmd/gdrive-bench/
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build $(GOFLAGS) -o dist/gdrive-bench-linux-arm64  ./cmd/gdrive-bench/
	@echo "Built:" && ls -lh dist/

clean:
	rm -rf dist/
	rm -f gdrive-bench gcs-bench
