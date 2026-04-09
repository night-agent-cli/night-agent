DYLIB      = guardian-intercept.dylib
DYLIB_SRC  = internal/intercept/csrc/guardian_intercept.c
BINARY     = guardian
HELPER_SRC = internal/intercept/testdata/exec-helper/main.c
HELPER     = internal/intercept/testdata/exec-helper/exec-helper
SHIM       = guardian-shim
SHIM_SRC   = internal/shim/csrc/guardian_shim.c

.PHONY: all build dylib shim helper test integration-test clean

all: dylib shim helper build

build:
	go build -o $(BINARY) ./cmd/guardian

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
