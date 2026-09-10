GO         ?= go
BIN        ?= svault.exe
PKG        ?= ./cmd/svault
DIST       ?= dist
VCPKG_ROOT ?= C:/Users/Eron/vcpkg
BASH       ?= C:/msys64/usr/bin/bash.exe

VC_INC := $(VCPKG_ROOT)/installed/x64-windows/include/sqlcipher
VC_LIB := $(VCPKG_ROOT)/installed/x64-windows/lib

# cgo needs the SQLCipher headers and import library from the vcpkg install.
export CGO_ENABLED := 1
export CGO_CFLAGS  := -I$(VC_INC)
export CGO_LDFLAGS := -L$(VC_LIB) -lsqlcipher

.PHONY: all build dlls run test fmt vet tidy clean

all: build

# build depends on dlls so a fresh clone bootstraps vcpkg/sqlcipher first.
build: dlls
	$(GO) build -o $(DIST)/$(BIN) $(PKG)

dlls:
	$(BASH) build.sh

run: build
	$(DIST)/$(BIN) $(ARGS)

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

clean:
	-$(BASH) -c "export PATH=/usr/bin:/bin:$$PATH; rm -rf $(DIST)"
