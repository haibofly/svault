#!/usr/bin/env bash
#
# Rebuild libsqlcipher.a from the SQLCipher amalgamation (sqlite3.c).
#
# This is the exact command used to produce the checked-in libsqlcipher.a,
# pinned so it can be reproduced on any machine.
#
# Requirements:
#   - MinGW-w64 gcc on PATH (e.g. the MSYS2 "UCRT64" environment)
#   - ../openssl/{include,lib} vendored in this repository (already included)
#
# Usage (from anywhere):
#   bash build.sh
#
set -euo pipefail

cd "$(dirname "$0")"

CC="${CC:-gcc}"
OPENSSL_INC="../openssl/include"
OPENSSL_LIB="../openssl/lib"

if ! command -v "$CC" >/dev/null 2>&1; then
  echo "error: C compiler '$CC' not found on PATH." >&2
  echo "       Install MSYS2 and add C:\\msys64\\ucrt64\\bin to PATH." >&2
  exit 1
fi

if [ ! -f sqlite3.c ]; then
  echo "error: sqlite3.c not found in $(pwd)" >&2
  exit 1
fi

if [ ! -f "$OPENSSL_LIB/libcrypto.a" ]; then
  echo "error: $OPENSSL_LIB/libcrypto.a not found." >&2
  exit 1
fi

echo ">> compiling sqlite3.c (SQLCipher amalgamation) with $CC"
"$CC" -O2 -c sqlite3.c -o sqlite3.o \
  -DSQLITE_HAS_CODEC \
  -DSQLCIPHER_CRYPTO_OPENSSL \
  -DSQLITE_TEMP_STORE=2 \
  -DSQLITE_THREADSAFE=1 \
  -DSQLITE_ENABLE_FTS5 \
  -DSQLITE_ENABLE_JSON1 \
  -I"$OPENSSL_INC"

echo ">> creating libsqlcipher.a"
ar rcs libsqlcipher.a sqlite3.o
rm -f sqlite3.o

echo ">> done: $(pwd)/libsqlcipher.a"
