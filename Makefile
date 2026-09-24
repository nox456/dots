build:
	@mkdir -p bin
	go build -o ./bin/dots ./cmd/dots
	@echo "Build complete"

install:
	@ln -s ./bin/dots $(HOME)/.local/bin/dots
