
.PHONY: ctrl cli build push clean

VERSION ?= latest
IMG_CTRL ?= hjmasha/mesh-ctrl
IMG_CLI ?= hjmasha/mesh-cli

cli:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o _output/mesh-cli cmd/cli/main.go

ctrl:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o _output/mesh-ctrl cmd/ctrl/main.go

build: ctrl cli
	docker build -f build/ctrl/Dockerfile . -t $(IMG_CTRL):$(VERSION) -t $(IMG_CTRL):latest
	docker build -f build/cli/Dockerfile . -t $(IMG_CLI):$(VERSION) -t $(IMG_CLI):latest

DOCKER_PUSH_RETRY = for i in 1 2 3; do docker push $$img && break || { echo "[WARN] push failed (attempt $$i/3), retrying in 5s..."; sleep 5; }; done

push: build
	@img=$(IMG_CTRL):$(VERSION); $(DOCKER_PUSH_RETRY)
	@img=$(IMG_CTRL):latest;    $(DOCKER_PUSH_RETRY)
	@img=$(IMG_CLI):$(VERSION); $(DOCKER_PUSH_RETRY)
	@img=$(IMG_CLI):latest;     $(DOCKER_PUSH_RETRY)

clean:
	rm -rf ./_output
