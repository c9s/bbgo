TARGET_ARCH ?= amd64
BUILD_DIR ?= build
BIN_DIR := $(BUILD_DIR)/bbgo
DIST_DIR ?= dist
GIT_DESC  = $$(git describe --long --tags)

OSX_APP_NAME = BBGO.app
OSX_APP_DIR = build/$(OSX_APP_NAME)
OSX_APP_CONTENTS_DIR = $(OSX_APP_DIR)/Contents
OSX_APP_RESOURCES_DIR = $(OSX_APP_CONTENTS_DIR)/Resources

all: $(BIN_DIR)
	go build -o $(BIN_DIR)/$@ ./cmd/$@

$(BIN_DIR):
	mkdir -p $@

bbgo-linux: $(BIN_DIR)
	GOOS=linux GOARCH=$(TARGET_ARCH) go build -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-darwin: $(BIN_DIR)
	GOOS=darwin GOARCH=$(TARGET_ARCH) go build -o $(BIN_DIR)/$@ ./cmd/bbgo

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR)

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
	go build -o $@ ./cmd/bbgo-desktop

desktop.osx: $(OSX_APP_CONTENTS_DIR)/MacOS/bbgo-desktop $(OSX_APP_CONTENTS_DIR)/Info.plist $(OSX_APP_RESOURCES_DIR)/icon.icns

desktop: desktop.osx

dist: static bbgo-linux bbgo-darwin desktop
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

pkged.go: frontend/out/index.html .FORCE
	pkger
	git commit pkged.go -m "pkger: update bundled static files"

static: frontend/out/index.html pkged.go

tools:
	GO111MODULES=off go get github.com/markbates/pkger/cmd/pkger

.PHONY: bbgo dist migrations static desktop .FORCE
