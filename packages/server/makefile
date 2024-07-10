# Makefile

# Directory for build output
BUILD_DIR := build
BINARY := $(BUILD_DIR)/server

# Source files
SRC := $(shell find . -name '*.go')

# Default target: build the binary
build: $(BINARY)

# Rule to build the binary
$(BINARY): $(SRC)
	@mkdir -p $(BUILD_DIR)
	go build -o $(BINARY) ./...

# Run the binary
run: build
	./$(BINARY)

dev:
	@find . -name '*.go' | entr make run

.PHONY: build run
