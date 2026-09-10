package session

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	crypt32            = syscall.NewLazyDLL("crypt32.dll")
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtect   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect = crypt32.NewProc("CryptUnprotectData")
	procLocalFree      = kernel32.NewProc("LocalFree")
)

// dataBlob mirrors the Win32 DATA_BLOB structure.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

// CRYPTPROTECT_UI_FORBIDDEN: never show UI, fail instead.
const cryptProtectUIForbidden = 0x1

func newBlob(data []byte) dataBlob {
	if len(data) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
}

// protect encrypts plain with DPAPI (current-user scope).
func protect(plain []byte) ([]byte, error) {
	in := newBlob(plain)
	var out dataBlob
	r, _, err := procCryptProtect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // szDataDescr
		0, // pOptionalEntropy
		0, // pvReserved
		0, // pPromptStruct
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	runtime.KeepAlive(plain)
	if r == 0 {
		return nil, wrap(err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return copyBlob(out), nil
}

// unprotect decrypts a DPAPI blob produced by protect.
func unprotect(blob []byte) ([]byte, error) {
	in := newBlob(blob)
	var out dataBlob
	r, _, err := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // ppszDataDescr
		0, // pOptionalEntropy
		0, // pvReserved
		0, // pPromptStruct
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	runtime.KeepAlive(blob)
	if r == 0 {
		return nil, wrap(err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return copyBlob(out), nil
}

func copyBlob(b dataBlob) []byte {
	if b.pbData == nil || b.cbData == 0 {
		return nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(b.pbData)), b.cbData)
	out := make([]byte, len(src))
	copy(out, src)
	return out
}

func wrap(err error) error {
	if err == nil || errors.Is(err, syscall.Errno(0)) {
		return errors.New("DPAPI call failed")
	}
	return err
}
