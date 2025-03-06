# Define a variable for the asdf executable
ASDF_BIN := $(shell command -v asdf 2>/dev/null)

# Define a variable for the scrab executable
SCRAB_BIN := $(shell command -v scrab 2>/dev/null)

# Default target
all: install_asdf

# Target to install asdf using Go if it's not installed
install_asdf:
ifndef ASDF_BIN
	@echo "asdf is not installed. Installing asdf using Go..."
	@go install github.com/asdf-vm/asdf/cmd/asdf@v0.16.0
	@echo "asdf installed successfully."
else
	@echo "asdf is already installed."
endif

# Target to install scrab using asdf if it's not installed
install_scrab: install_asdf
ifndef SCRAB_BIN
	@asdf plugin add scarb || true
	@if ! asdf list scarb | grep -q "2.10.1"; then \
		asdf install scarb 2.10.1; \
		@asdf set -u scarb 2.10.1; \
	else \
		echo "scarb 2.10.1 is already installed."; \
	fi
else
	@echo "scrab is already installed."
endif

build: ## `scarb build` generates json with `abi` field as array, we need to convert it to string
	scarb build
	for file in target/dev/*.contract_class.json; do \
		jq '.abi = ( .abi | tostring )' $$file > tmp.json; \
		mv tmp.json $$file; \
	done

clean: ## Clean the project
	scarb clean
