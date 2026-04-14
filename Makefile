DYLIB      = guardian-intercept.dylib
DYLIB_SRC  = internal/intercept/csrc/guardian_intercept.c
BINARY     = nightagent
HELPER_SRC = internal/intercept/testdata/exec-helper/main.c
HELPER     = internal/intercept/testdata/exec-helper/exec-helper
SHIM       = guardian-shim
SHIM_SRC   = internal/shim/csrc/guardian_shim.c

ENDPOINT_PKG = github.com/pietroperona/night-agent/internal/cloudconfig.defaultEndpoint
ENDPOINT_PROD    = https://api.nightagent.dev
ENDPOINT_STAGING = https://staging.api.nightagent.dev
ENDPOINT_DEV     = http://localhost:8000

.PHONY: all build build-dev build-staging dylib shim helper test integration-test clean

all: dylib shim helper build

# Produzione — endpoint prod hardcodato
build:
	go build -ldflags "-X '$(ENDPOINT_PKG)=$(ENDPOINT_PROD)'" -o $(BINARY) ./cmd/guardian

# Staging — endpoint staging hardcodato
build-staging:
	go build -ldflags "-X '$(ENDPOINT_PKG)=$(ENDPOINT_STAGING)'" -o $(BINARY) ./cmd/guardian

# Dev locale — punta a localhost:8000
build-dev:
	go build -ldflags "-X '$(ENDPOINT_PKG)=$(ENDPOINT_DEV)'" -o $(BINARY) ./cmd/guardian

# Installa il binario dev in /usr/local/bin (sovrascrive quello prod)
install-dev: build-dev
	cp $(BINARY) $(HOME)/.local/bin/$(BINARY)
	@echo "installato: $(HOME)/.local/bin/$(BINARY) → $(ENDPOINT_DEV)"

dylib:
	clang -dynamiclib \
		-o $(DYLIB) $(DYLIB_SRC) \
		-Wall -Wextra \
		-Wno-unused-parameter \
		-current_version 1.0 \
		-compatibility_version 1.0
	@echo "dylib compilata: $(DYLIB)"

shim:
	clang -o $(SHIM) $(SHIM_SRC) \
		-Wall -Wextra \
		-Wno-unused-parameter
	@echo "shim compilato: $(SHIM)"

helper:
	clang -o $(HELPER) $(HELPER_SRC) -Wall
	@echo "exec-helper compilato: $(HELPER)"

test:
	go test ./...

integration-test: dylib shim helper
	go test -tags integration ./internal/intercept/... -v

clean:
	rm -f $(BINARY) $(DYLIB) $(SHIM) $(HELPER)
