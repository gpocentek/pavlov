.PHONY: pavlov test

pavlov:
	go build -o pavlov ./cmd/pavlov

test:
	go clean -testcache
	go test -v ./...
