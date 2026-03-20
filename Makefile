
.PHONY: ctrl cli webhook build build-webhook push push-webhook clean

VERSION ?= latest
IMG_CTRL ?= hjmasha/mesh-ctrl
IMG_CLI ?= hjmasha/mesh-cli
IMG_WEBHOOK ?= hjmasha/mesh-webhook

cli:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o _output/mesh-cli cmd/cli/main.go

ctrl:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o _output/mesh-ctrl cmd/ctrl/main.go

webhook:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o _output/webhook cmd/webhook/main.go

build: ctrl cli
	docker build -f build/ctrl/Dockerfile . -t $(IMG_CTRL):$(VERSION) -t $(IMG_CTRL):latest
	docker build -f build/cli/Dockerfile . -t $(IMG_CLI):$(VERSION) -t $(IMG_CLI):latest

build-webhook: webhook
	docker build -f build/webhook/Dockerfile . -t $(IMG_WEBHOOK):$(VERSION) -t $(IMG_WEBHOOK):latest

push: build
	docker push $(IMG_CTRL):$(VERSION)
	docker push $(IMG_CTRL):latest
	docker push $(IMG_CLI):$(VERSION)
	docker push $(IMG_CLI):latest

push-webhook: build-webhook
	docker push $(IMG_WEBHOOK):$(VERSION)
	docker push $(IMG_WEBHOOK):latest

clean:
	rm -rf ./_output
