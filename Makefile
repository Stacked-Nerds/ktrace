run:
	go run ./cmd/ktrace

build:
	go build -o bin/ktrace ./cmd/ktrace

install:
	go install ./cmd/ktrace

test:
	go test -race ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

docker-build:
	docker build -t ghcr.io/stacked-nerds/ktrace:local .

docker-run:
	docker run --rm -v "$(HOME)/.kube:/home/ktrace/.kube:ro" ghcr.io/stacked-nerds/ktrace:local deployment frontend

fmt:
	go fmt ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/ dist/

# Optional manual verification against a real cluster (not run in CI).
test-cluster:
	@echo "Apply example manifests, then run:"
	@echo "  ktrace deployment frontend -n production"
	@echo "See examples/deployment-failure/ for setup."

.PHONY: run build install test vet lint fmt tidy clean test-cluster docker-build docker-run
