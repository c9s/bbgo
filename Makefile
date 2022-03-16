TARGET_ARCH ?= amd64
BUILD_DIR ?= build
BIN_DIR := $(BUILD_DIR)/bbgo
DIST_DIR ?= dist
GIT_DESC := $(shell git describe --tags)

VERSION ?= $(shell git describe --tags)

OSX_APP_NAME = BBGO.app
OSX_APP_DIR = build/$(OSX_APP_NAME)
OSX_APP_CONTENTS_DIR = $(OSX_APP_DIR)/Contents
OSX_APP_RESOURCES_DIR = $(OSX_APP_CONTENTS_DIR)/Resources
OSX_APP_CODESIGN_IDENTITY ?=

# OSX_APP_GUI ?= lorca
OSX_APP_GUI ?= webview

FRONTEND_EXPORT_DIR = frontend/out

all: bbgo-linux bbgo-darwin

$(BIN_DIR):
	mkdir -p $@


# build native bbgo
bbgo: static
	go build -tags web,release -o $(BIN_DIR)/bbgo ./cmd/bbgo

# build native bbgo (slim version)
bbgo-slim:
	go build -tags release -o $(BIN_DIR)/$@ ./cmd/bbgo

# build cross-compile linux bbgo
bbgo-linux: bbgo-linux-amd64 bbgo-linux-arm64

bbgo-linux-amd64: $(BIN_DIR) pkg/server/assets.go
	GOOS=linux GOARCH=amd64 go build -tags web,release -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-linux-arm64: $(BIN_DIR) pkg/server/assets.go
	GOOS=linux GOARCH=arm64 go build -tags web,release -o $(BIN_DIR)/$@ ./cmd/bbgo

# build cross-compile linux bbgo (slim version)
bbgo-slim-linux: bbgo-slim-linux-amd64 bbgo-slim-linux-arm64

bbgo-slim-linux-amd64: $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -tags release -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-slim-linux-arm64: $(BIN_DIR)
	GOOS=linux GOARCH=arm64 go build -tags release -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-darwin: bbgo-darwin-arm64 bbgo-darwin-amd64

bbgo-darwin-arm64: $(BIN_DIR) pkg/server/assets.go
	GOOS=darwin GOARCH=arm64 go build -tags web,release -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-darwin-amd64: $(BIN_DIR) pkg/server/assets.go
	GOOS=darwin GOARCH=amd64 go build -tags web,release -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-slim-darwin-arm64: $(BIN_DIR)
	GOOS=darwin GOARCH=arm64 go build -tags release -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-slim-darwin-amd64: $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 go build -tags release -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-slim-darwin: bbgo-slim-darwin-amd64 bbgo-slim-darwin-arm64

# build native bbgo
bbgo-dnum: static
	go build -tags web,release,dnum -o $(BIN_DIR)/bbgo ./cmd/bbgo

# build native bbgo (slim version)
bbgo-slim-dnum:
	go build -tags release,dnum -o $(BIN_DIR)/$@ ./cmd/bbgo

# build cross-compile linux bbgo
bbgo-dnum-linux: bbgo-dnum-linux-amd64 bbgo-dnum-linux-arm64

bbgo-dnum-linux-amd64: $(BIN_DIR) pkg/server/assets.go
	GOOS=linux GOARCH=amd64 go build -tags web,release,dnum -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-dnum-linux-arm64: $(BIN_DIR) pkg/server/assets.go
	GOOS=linux GOARCH=arm64 go build -tags web,release,dnum -o $(BIN_DIR)/$@ ./cmd/bbgo

# build cross-compile linux bbgo (slim version)
bbgo-slim-dnum-linux: bbgo-slim-dnum-linux-amd64 bbgo-slim-dnum-linux-arm64

bbgo-slim-dnum-linux-amd64: $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -tags release,dnum -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-slim-dnum-linux-arm64: $(BIN_DIR)
	GOOS=linux GOARCH=arm64 go build -tags release,dnum -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-dnum-darwin: bbgo-dnum-darwin-arm64 bbgo-dnum-darwin-amd64

bbgo-dnum-darwin-arm64: $(BIN_DIR) pkg/server/assets.go
	GOOS=darwin GOARCH=arm64 go build -tags web,release,dnum -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-dnum-darwin-amd64: $(BIN_DIR) pkg/server/assets.go
	GOOS=darwin GOARCH=amd64 go build -tags web,release,dnum -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-slim-dnum-darwin-arm64: $(BIN_DIR)
	GOOS=darwin GOARCH=arm64 go build -tags release,dnum -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-slim-dnum-darwin-amd64: $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 go build -tags release,dnum -o $(BIN_DIR)/$@ ./cmd/bbgo

bbgo-slim-dnum-darwin: bbgo-slim-dnum-darwin-amd64 bbgo-slim-dnum-darwin-arm64


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
	go build -tags web -o $@ ./cmd/bbgo-$(OSX_APP_GUI)

desktop-osx: $(OSX_APP_CONTENTS_DIR)/MacOS/bbgo-desktop $(OSX_APP_CONTENTS_DIR)/Info.plist $(OSX_APP_RESOURCES_DIR)/icon.icns
	if [[ -n "$(OSX_APP_CODESIGN_IDENTITY)" ]] ; then codesign --deep --force --verbose --sign "$(OSX_APP_CODESIGN_IDENTITY)" $(OSX_APP_DIR) \
		&& codesign --verify -vvvv $(OSX_APP_DIR) ; fi

desktop: desktop-osx

$(DIST_DIR)/$(VERSION):
	mkdir -p $(DIST_DIR)/$(VERSION)

$(DIST_DIR)/$(VERSION)/bbgo-slim-$(VERSION)-%.tar.gz: bbgo-slim-% $(DIST_DIR)/$(VERSION)
	tar -C $(BIN_DIR) -cvzf $@ $<
ifeq ($(SIGN),1)
	gpg --yes --detach-sign --armor $@
endif

$(DIST_DIR)/$(VERSION)/bbgo-$(VERSION)-%.tar.gz: bbgo-% $(DIST_DIR)/$(VERSION)
	tar -C $(BIN_DIR) -cvzf $@ $<
ifeq ($(SIGN),1)
	gpg --yes --detach-sign --armor $@
endif

$(DIST_DIR)/$(VERSION)/bbgo-slim-dnum-$(VERSION)-%.tar.gz: bbgo-slim-dnum-% $(DIST_DIR)/$(VERSION)
	tar -C $(BIN_DIR) -cvzf $@ $<
ifeq ($(SIGN),1)
	gpg --yes --detach-sign --armor $@
endif

$(DIST_DIR)/$(VERSION)/bbgo-dnum-$(VERSION)-%.tar.gz: bbgo-dnum-% $(DIST_DIR)/$(VERSION)
	tar -C $(BIN_DIR) -cvzf $@ $<
ifeq ($(SIGN),1)
	gpg --yes --detach-sign --armor $@
endif

dist-bbgo-linux: \
	$(DIST_DIR)/$(VERSION)/bbgo-$(VERSION)-linux-arm64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-$(VERSION)-linux-amd64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-slim-$(VERSION)-linux-arm64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-slim-$(VERSION)-linux-amd64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-dnum-$(VERSION)-linux-arm64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-dnum-$(VERSION)-linux-amd64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-slim-dnum-$(VERSION)-linux-arm64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-slim-dnum-$(VERSION)-linux-amd64.tar.gz

dist-bbgo-darwin: \
	$(DIST_DIR)/$(VERSION)/bbgo-$(VERSION)-darwin-arm64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-$(VERSION)-darwin-amd64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-slim-$(VERSION)-darwin-arm64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-slim-$(VERSION)-darwin-amd64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-dnum-$(VERSION)-darwin-arm64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-dnum-$(VERSION)-darwin-amd64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-slim-dnum-$(VERSION)-darwin-arm64.tar.gz \
	$(DIST_DIR)/$(VERSION)/bbgo-slim-dnum-$(VERSION)-darwin-amd64.tar.gz

dist: static dist-bbgo-linux dist-bbgo-darwin desktop

pkg/version/version.go: .FORCE
	BUILD_FLAGS="release" bash utils/generate-version-file.sh > $@

pkg/version/dev.go: .FORCE
	BUILD_FLAGS="!release" VERSION_SUFFIX="-dev" bash utils/generate-version-file.sh > $@

dev-version: pkg/version/dev.go
	git add $<
	git commit $< -m "update dev build version"

cmd-doc: .FORCE
	go run ./cmd/update-doc
	git add -v doc/commands
	git commit -m "update command doc files" doc/commands || true

version: pkg/version/version.go pkg/version/dev.go migrations cmd-doc
	git add $< $(word 2,$^)
	git commit $< $(word 2,$^) -m "bump version to $(VERSION)" || true
	[[ -e doc/release/$(VERSION).md ]] || (echo "file doc/release/$(VERSION).md does not exist" ; exit 1)
	git add -v doc/release/$(VERSION).md && git commit doc/release/$(VERSION).md -m "add $(VERSION) release note" || true
	git tag -f $(VERSION)
	git push origin HEAD
	git push -f origin $(VERSION)

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

frontend/node_modules:
	cd frontend && yarn install

frontend/out/index.html: frontend/node_modules
	cd frontend && yarn export

pkg/server/assets.go: frontend/out/index.html
	go run ./util/embed -package server -output $@ $(FRONTEND_EXPORT_DIR)

embed: pkg/server/assets.go

static: frontend/out/index.html pkg/server/assets.go

.PHONY: bbgo bbgo-slim-darwin bbgo-slim-darwin-amd64 bbgo-slim-darwin-arm64 bbgo-darwin version dist pack migrations static embed desktop  .FORCE

protobuf:
	protoc -I=$(PWD)/pkg/pb --go_out=$(PWD)/pkg/pb $(PWD)/pkg/pb/bbgo.proto

protobuf-py:
	python -m grpc_tools.protoc -I$(PWD)/pkg/pb --python_out=$(PWD)/python/bbgo --grpc_python_out=$(PWD)/python/bbgo $(PWD)/pkg/pb/bbgo.proto
