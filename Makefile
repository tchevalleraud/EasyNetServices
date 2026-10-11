VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
IMAGE   ?= easynetservices:local

PKG     := github.com/tchevalleraud/EasyNetServices/internal/version
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

.PHONY: all web build test lint run docker up down logs clean

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

up: ## Build and start the local development stack (compose.dev.yaml)
	docker compose -f compose.dev.yaml up -d --build

down: ## Stop the local development stack
	docker compose -f compose.dev.yaml down

logs: ## Follow the logs of the local development stack
	docker compose -f compose.dev.yaml logs -f

clean:
	rm -rf bin web/dist
