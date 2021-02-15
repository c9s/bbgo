TARGET_ARCH ?= amd64
BUILD_DIR ?= build
BIN_DIR := $(BUILD_DIR)/bbgo
DIST_DIR ?= dist
GIT_DESC  = $$(git describe --long --tags)

OSX_APP_NAME = BBGO.app
OSX_APP_DIR = build/$(OSX_APP_NAME)
OSX_APP_CONTENTS_DIR = $(OSX_APP_DIR)/Contents
OSX_APP_RESOURCES_DIR = $(OSX_APP_CONTENTS_DIR)/Resources

FRONTEND_EXPORT_DIR = frontend/out

all: $(BIN_DIR)
	go build -tags web -o $(BIN_DIR)/$@ ./cmd/$@

$(BIN_DIR):
	mkdir -p $@

bbgo-linux: $(BIN_DIR)
	GOOS=linux GOARCH=$(TARGET_ARCH) go build -tags web -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-darwin: $(BIN_DIR)
	GOOS=darwin GOARCH=$(TARGET_ARCH) go build -tags web -o $(BIN_DIR)/$@ ./cmd/bbgo

bbog-darwin-slim: $(BIN_DIR)
	GOOS=darwin GOARCH=$(TARGET_ARCH) go build -o $(BIN_DIR)/$@ ./cmd/bbgo

bbog-linux-slim: $(BIN_DIR)
	GOOS=linux GOARCH=$(TARGET_ARCH) go build -o $(BIN_DIR)/$@ ./cmd/bbgo

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(FRONTEND_EXPORT_DIR)

$(OSX_APP_CONTENTS_DIR):
	mkdir -p $@

$(OSX_APP_CONTENTS_DIR)/MacOS: $(OSX_APP_CONTENTS_DIR)
	mkdir -p $@

$(OSX_APP_RESOURCES_DIR): $(OSX_APP_CONTENTS_DIR)
	mkdir -p $@

$(OSX_APP_RESOURCES_DIR)/icon.icns: $(OSX_APP_RESOURCES_DIR)
	cp -v desktop/icons/icon.icns $@

$(OSX_APP_CONTENTS_DIR)/Info.plist: $(OSX_APP_CONTENTS_DIR)
	bash desktop/build-osx-info-plist.sh > $@

$(OSX_APP_CONTENTS_DIR)/MacOS/bbgo-desktop: $(OSX_APP_CONTENTS_DIR)/MacOS .FORCE
	go build -tags web -o $@ ./cmd/bbgo-desktop

desktop-osx: $(OSX_APP_CONTENTS_DIR)/MacOS/bbgo-desktop $(OSX_APP_CONTENTS_DIR)/Info.plist $(OSX_APP_RESOURCES_DIR)/icon.icns

desktop: desktop-osx

dist: static bbgo-linux bbgo-linux-slim bbgo-darwin bbgo-darwin-slim desktop
	mkdir -p $(DIST_DIR)
	tar -C $(BUILD_DIR) -cvzf $(DIST_DIR)/bbgo-$$(git describe --tags).tar.gz .

migrations:
	rockhopper compile --config rockhopper_mysql.yaml --output pkg/migrations/mysql
	rockhopper compile --config rockhopper_sqlite.yaml --output pkg/migrations/sqlite3
	git add -v pkg/migrations && git commit -m "compile and update migration package" pkg/migrations || true

docker:
	GOPATH=$(PWD)/_mod go mod download
	docker build --build-arg GO_MOD_CACHE=_mod --tag yoanlin/bbgo .
	bash -c "[[ -n $(DOCKER_TAG) ]] && docker tag yoanlin/bbgo yoanlin/bbgo:$(DOCKER_TAG)"

docker-push:
	docker push yoanlin/bbgo
	bash -c "[[ -n $(DOCKER_TAG) ]] && docker push yoanlin/bbgo:$(DOCKER_TAG)"

frontend/out/index.html: .FORCE
	(cd frontend && yarn export)

pkg/server/assets.go: frontend/out/index.html .FORCE
	go run ./util/embed -package server -output $@ $(FRONTEND_EXPORT_DIR)
	gofmt -w pkg/server/assets.go
	git add -v $@
	git commit $@ -m "assets: update embedded static files"

embed: pkg/server/assets.go

static: frontend/out/index.html pkg/server/assets.go

.PHONY: bbgo dist migrations static embed desktop .FORCE
