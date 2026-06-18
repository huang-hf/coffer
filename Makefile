.PHONY: build install test clean

# Build the coffer binary
build:
	go build -o coffer ./cmd/coffer

# Install to /usr/local/bin (requires sudo)
install: build
	sudo mv coffer /usr/local/bin/
	sudo chmod +x /usr/local/bin/coffer

# Install to user local bin (no sudo required)
install-user: build
	mkdir -p ~/bin
	cp coffer ~/bin/
	chmod +x ~/bin/coffer
	@echo "Added ~/bin to PATH in ~/.zshrc"
	@echo 'export PATH="$$HOME/bin:$$PATH"' >> ~/.zshrc

# Run all tests
test:
	go test ./... -v

# Clean build artifacts
clean:
	rm -f coffer
	rm -f ~/bin/coffer

# Show help
help:
	@echo "Usage:"
	@echo "  make build        - Build coffer binary"
	@echo "  make install      - Install to /usr/local/bin (requires sudo)"
	@echo "  make install-user - Install to ~/bin (no sudo)"
	@echo "  make test         - Run all tests"
	@echo "  make clean        - Remove build artifacts"
