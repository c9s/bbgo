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

.PHONY: dist
