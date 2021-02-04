TARGET_ARCH ?= amd64
BUILD_DIR ?= build
BIN_DIR := $(BUILD_DIR)/bbgo
DIST_DIR ?= dist
GIT_DESC  = $$(git describe --long --tags)

all: bbgo

$(BIN_DIR):
	mkdir -p $@

bin-dir: $(BIN_DIR)

bbgo-linux: $(BIN_DIR)
	GOOS=linux GOARCH=$(TARGET_ARCH) go build -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-darwin:
	GOOS=darwin GOARCH=$(TARGET_ARCH) go build -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo:
	go build -o $(BIN_DIR)/$@ ./cmd/$@

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR)

dist: bin-dir bbgo-linux bbgo-darwin
	mkdir -p $(DIST_DIR)
	tar -C $(BUILD_DIR) -cvzf $(DIST_DIR)/bbgo-$$(git describe --tags).tar.gz .

migrations:
	rockhopper compile --config rockhopper.yaml --output pkg/migrations
	git add -v pkg/migrations
	git commit -m "Update migration package" pkg/migrations

docker:
	GOPATH=$(PWD)/_mod go mod download
	docker build --build-arg GO_MOD_CACHE=_mod --tag yoanlin/bbgo .
	bash -c "[[ -n $(DOCKER_TAG) ]] && docker tag yoanlin/bbgo yoanlin/bbgo:$(DOCKER_TAG)"


docker-push:
	docker push yoanlin/bbgo
	bash -c "[[ -n $(DOCKER_TAG) ]] && docker push yoanlin/bbgo:$(DOCKER_TAG)"

frontend/out/index.html: .FORCE
	(cd frontend && yarn export)

pkged.go: frontend/out/index.html
	pkger
	git commit pkged.go -m "pkger: update bundled static files"

static: frontend/out/index.html pkged.go

build/BBGO.app/Contents/MacOS/bbgo-desktop: .FORCE
	mkdir -p $(dir $@)
	go build -o $@ ./cmd/bbgo-desktop

desktop: static build/BBGO.app/Contents/MacOS/bbgo-desktop
	# bash desktop/build-darwin.sh

tools:
	GO111MODULES=off go get github.com/markbates/pkger/cmd/pkger

.PHONY: dist migrations static desktop .FORCE
