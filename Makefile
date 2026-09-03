.PHONY: build test run clean service-install service-uninstall

BINARY_NAME=tailwake

build:
	@echo "🔨 Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) main.go
	@echo "✅ Build complete: ./$(BINARY_NAME)"

test:
	@echo "🧪 Running unit tests..."
	go test -v ./...

run: build
	./$(BINARY_NAME) serve --config config.example.json

service-install: build
	./$(BINARY_NAME) service install --config $(PWD)/config.json

service-uninstall:
	./$(BINARY_NAME) service uninstall

clean:
	rm -f $(BINARY_NAME)
	@echo "🧹 Cleaned build artifacts."
