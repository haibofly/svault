#!/usr/bin/env bash
#
# Prepare the runtime DLLs required by svault.exe.
#
# svault.exe links the official SQLCipher built by vcpkg (dynamic linking).
# This script makes sure vcpkg and sqlcipher are available, then copies the
# DLLs next to the executable in dist/.
#
# Environment variables:
#   VCPKG_ROOT   vcpkg location (default: C:/Users/Eron/vcpkg)
#   HTTPS_PROXY  set this if vcpkg needs a proxy to download/build packages
#
# Requirements for the first run:
#   - git and a network connection (to clone/bootstrap vcpkg)
#   - Visual Studio Build Tools (vcpkg's x64-windows triplet uses MSVC)
#
set -euo pipefail

# When launched from cmd/PowerShell, MSYS2's coreutils (dirname, grep, cp, ...)
# are not on PATH yet. Make sure the MSYS2 userland is available.
export PATH="/usr/bin:/bin:$PATH"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DIST="$SCRIPT_DIR/dist"
TRIPLET="x64-windows"

VCPKG_ROOT="${VCPKG_ROOT:-C:/Users/Eron/vcpkg}"
VCPKG_ROOT="${VCPKG_ROOT//\\//}"

VCPKG_EXE="$VCPKG_ROOT/vcpkg.exe"
BIN_DIR="$VCPKG_ROOT/installed/$TRIPLET/bin"

DLLS=(sqlcipher.dll libcrypto-3-x64.dll libssl-3-x64.dll)

echo ">> vcpkg root: $VCPKG_ROOT"

# 1. Ensure vcpkg is present.
if [ ! -f "$VCPKG_EXE" ]; then
  echo ">> vcpkg not found; cloning into $VCPKG_ROOT"
  git clone --depth 1 https://github.com/microsoft/vcpkg "$VCPKG_ROOT"
  echo ">> bootstrapping vcpkg"
  bash "$VCPKG_ROOT/bootstrap-vcpkg.sh" -disableMetrics
else
  echo ">> vcpkg found"
fi

# 2. Ensure sqlcipher is installed.
if "$VCPKG_EXE" list | grep -q "sqlcipher:$TRIPLET"; then
  echo ">> sqlcipher:$TRIPLET already installed"
else
  echo ">> installing sqlcipher:$TRIPLET (first build can take several minutes)"
  "$VCPKG_EXE" install "sqlcipher:$TRIPLET"
fi

# 3. Copy the runtime DLLs into dist/.
if [ ! -d "$BIN_DIR" ]; then
  echo "error: $BIN_DIR not found (did the vcpkg install succeed?)" >&2
  exit 1
fi

mkdir -p "$DIST"
for dll in "${DLLS[@]}"; do
  if [ ! -f "$BIN_DIR/$dll" ]; then
    echo "error: $BIN_DIR/$dll not found" >&2
    exit 1
  fi
  cp -f "$BIN_DIR/$dll" "$DIST/"
  echo "   copied $dll"
done

echo ">> runtime DLLs are in $DIST"
echo "   note: target machines also need the MSVC runtime"
echo "         (VCRUNTIME140.dll, part of the VC++ Redistributable)"
