.PHONY: build clean

BINARY_NAME=hello
BUILD_DIR=bin
LDFLAGS="-w -s -X main.AppName=$(BINARY_NAME)"

clean:
	@rm -rf $(BUILD_DIR)

build: clean
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) -ldflags $(LDFLAGS)
