VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
IMAGE   ?= easynetservices:local

PKG     := github.com/tchevalleraud/EasyNetServices/internal/version
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

.PHONY: all web build test lint run docker clean

all: build

web: ## Build the web interface
	cd web && npm ci && npm run build

build: web ## Build the server binary with the embedded web interface
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/easynetservices ./cmd/easynetservices

web/dist:
	mkdir -p web/dist && touch web/dist/.keep

test: web/dist ## Run the backend tests
	go vet ./...
	go test ./...

lint: web/dist ## Check formatting and lint both backend and frontend
	test -z "$$(gofmt -l .)"
	cd web && npm run lint

run: build ## Run the server locally with data in ./data
	ENS_DATA_DIR=./data ./bin/easynetservices serve

docker: ## Build the Docker image for the local architecture
	docker buildx build --load -t $(IMAGE) \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(DATE) .

clean:
	rm -rf bin web/dist
