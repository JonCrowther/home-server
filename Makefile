APP            := home-server

# Docker Hub username; required for `make push`. Override per-invocation
# (make push DOCKER_USER=you) or export it in your shell.
DOCKER_USER    ?= princelilac
IMAGE          ?= $(if $(DOCKER_USER),docker.io/$(DOCKER_USER)/$(APP),$(APP))

TAG            ?= latest

# Platform for local `make image` (loaded into your Docker) vs. what `make push` publishes.
LOCAL_PLATFORM ?= linux/arm64
PLATFORMS      ?= linux/arm64

PORT           ?= 8080

# File holding runtime secrets (e.g. LIFX_TOKEN); passed to `docker run --env-file`.
# Keep it out of git and the image (see .gitignore / .dockerignore).
ENV_FILE       ?= .env

# Home Assistant host for `make app-deploy`, reached through its Terminal & SSH app.
HA_HOST        ?= homeassistant.local
HA_APP_DIR     ?= /root/local_apps/home_server

.DEFAULT_GOAL := help
.PHONY: help build build-debug run test vet image image-run push app-deploy clean

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the Go binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$(APP) .

build-debug: ## Build bin/ binary that logs at DEBUG by default (e.g. every score poll)
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.defaultLogLevel=DEBUG" -o bin/$(APP) .

run: ## Run the server locally with go run (loads $(ENV_FILE))
	@test -f $(ENV_FILE) || { echo "$(ENV_FILE) not found; copy .env.example to $(ENV_FILE) and set LIFX_TOKEN"; exit 1; }
	set -a; . ./$(ENV_FILE); set +a; PORT=$(PORT) go run .

test: ## Run Go tests
	go test ./...

vet: ## Run go vet
	go vet ./...

image: ## Build the Docker image and load it into local Docker
	docker buildx build --platform $(LOCAL_PLATFORM) -t $(IMAGE):$(TAG) --load .

image-run: image ## Build and run the image locally on $(PORT)
	@test -f $(ENV_FILE) || { echo "$(ENV_FILE) not found; copy .env.example to $(ENV_FILE) and set LIFX_TOKEN"; exit 1; }
	docker run --rm --env-file $(ENV_FILE) -p $(PORT):8080 $(IMAGE):$(TAG)

push: ## Build for $(PLATFORMS) and push to Docker Hub (needs DOCKER_USER)
	@test -n "$(DOCKER_USER)" || { echo "DOCKER_USER is not set, e.g. make push DOCKER_USER=yourname"; exit 1; }
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE):$(TAG) --push .
	@echo "Pushed $(IMAGE):$(TAG)"

app-deploy: ## Copy the Home Assistant app and source to $(HA_HOST)
	COPYFILE_DISABLE=1 tar -cz go.mod $(wildcard go.sum) $(wildcard *.go) lifx nhl -C app config.yaml Dockerfile \
		| ssh root@$(HA_HOST) 'rm -rf $(HA_APP_DIR) && mkdir -p $(HA_APP_DIR) && tar -xzf - -C $(HA_APP_DIR)'
	@echo "Copied to $(HA_HOST):$(HA_APP_DIR). Install, or Rebuild, it from Settings > Apps."

clean: ## Remove build output
	rm -rf bin/
