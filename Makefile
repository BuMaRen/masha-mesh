
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

push: build
	docker push $(IMG_CTRL):$(VERSION)
	docker push $(IMG_CTRL):latest
	docker push $(IMG_CLI):$(VERSION)
	docker push $(IMG_CLI):latest

clean:
	rm -rf ./_output
