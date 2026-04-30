# ── Platform detection ──────────────────────────────────────────────
ifeq ($(OS), Windows_NT)
  PLATFORM      = windows
  CC            = gcc
  DYLIB_EXT     = dll
  DYLIB_LDFLAGS = -shared
  EXTRA_LDFLAGS = -lws2_32
  EXE           = .exe
  CLEAN_CMD     = del /f
else
  UNAME        := $(shell uname)
  EXTRA_LDFLAGS =
  EXE           =
  CLEAN_CMD     = rm -f

  ifeq ($(UNAME), Darwin)
    PLATFORM      = darwin
    CC            = clang
    DYLIB_EXT     = dylib
    DYLIB_LDFLAGS = -dynamiclib \
                    -current_version 1.0 \
                    -compatibility_version 1.0
  else
    PLATFORM      = linux
    CC            = clang
    DYLIB_EXT     = so
    DYLIB_LDFLAGS = -shared -fPIC
  endif
endif

# ── Variables ────────────────────────────────────────────────────────
DYLIB      = guardian-intercept.$(DYLIB_EXT)
DYLIB_SRC  = internal/intercept/csrc/guardian_intercept.c
BINARY     = nightagent$(EXE)
HELPER_SRC = internal/intercept/testdata/exec-helper/main.c
HELPER     = internal/intercept/testdata/exec-helper/exec-helper$(EXE)
SHIM       = guardian-shim$(EXE)
SHIM_SRC   = internal/shim/csrc/guardian_shim.c

ENDPOINT_PKG     = github.com/pietroperona/night-agent/internal/cloudconfig.defaultEndpoint
ENDPOINT_PROD    = https://api.nightagent.dev
ENDPOINT_STAGING = https://staging.api.nightagent.dev
ENDPOINT_DEV     = http://localhost:8000

.PHONY: all build build-dev build-staging dylib shim helper test integration-test clean install-dev

all: dylib shim helper build

# ── Build targets ────────────────────────────────────────────────────

build:
	go build -ldflags "-X '$(ENDPOINT_PKG)=$(ENDPOINT_PROD)'" -o $(BINARY) ./cmd/guardian

build-staging:
	go build -ldflags "-X '$(ENDPOINT_PKG)=$(ENDPOINT_STAGING)'" -o $(BINARY) ./cmd/guardian

build-dev:
	go build -ldflags "-X '$(ENDPOINT_PKG)=$(ENDPOINT_DEV)'" -o $(BINARY) ./cmd/guardian

# install-dev: disponibile solo su Unix (Linux / macOS)
ifeq ($(OS), Windows_NT)
install-dev:
	@echo "install-dev non supportato su Windows"
else
install-dev: build-dev
	cp $(BINARY) $(HOME)/.local/bin/$(BINARY)
	@echo "installato: $(HOME)/.local/bin/$(BINARY) -> $(ENDPOINT_DEV)"
endif

# ── C compilation ────────────────────────────────────────────────────

dylib:
	$(CC) $(DYLIB_LDFLAGS) \
		-o $(DYLIB) $(DYLIB_SRC) \
		-Wall -Wextra \
		-Wno-unused-parameter \
		$(EXTRA_LDFLAGS)
	@echo "dylib compilata: $(DYLIB) [$(PLATFORM)]"

shim:
	$(CC) -o $(SHIM) $(SHIM_SRC) \
		-Wall -Wextra \
		-Wno-unused-parameter \
		$(EXTRA_LDFLAGS)
	@echo "shim compilato: $(SHIM)"

helper:
	$(CC) -o $(HELPER) $(HELPER_SRC) -Wall
	@echo "exec-helper compilato: $(HELPER)"

# ── Tests ─────────────────────────────────────────────────────────────

test:
	go test ./...

integration-test: dylib shim helper
	go test -tags integration ./internal/intercept/... -v

# ── Clean ─────────────────────────────────────────────────────────────

clean:
	$(CLEAN_CMD) $(BINARY) $(DYLIB) $(SHIM) $(HELPER)
