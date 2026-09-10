package sqlcipher

/*
#cgo CFLAGS: -DSQLITE_HAS_CODEC -DSQLCIPHER_CRYPTO_OPENSSL -I${SRCDIR}/../../third_party/sqlcipher -I${SRCDIR}/../../third_party/openssl/include
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/sqlcipher -L${SRCDIR}/../../third_party/openssl/lib -lsqlcipher -lssl -lcrypto -lws2_32 -lgdi32 -lcrypt32 -lbcrypt -luser32 -ladvapi32 -lole32 -lshell32 -lwldap32 -static
#include <sqlite3.h>
#include <stdlib.h>

static int sv_bind_text(sqlite3_stmt *stmt, int idx, const char *val, int n) {
	return sqlite3_bind_text(stmt, idx, val, n, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

const (
	rcOK   = 0
	rcRow  = 100
	rcDone = 101
	colNull = 5
)

// DB wraps a SQLCipher database handle.
type DB struct {
	ptr *C.sqlite3
}

// Open opens (or creates) a database file. The key is not applied yet; call Key.
func Open(path string) (*DB, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var db *C.sqlite3
	if rc := C.sqlite3_open(cpath, &db); rc != rcOK {
		msg := "unable to open database"
		if db != nil {
			msg = C.GoString(C.sqlite3_errmsg(db))
			C.sqlite3_close(db)
		}
		return nil, errors.New(msg)
	}
	return &DB{ptr: db}, nil
}

// Key applies the encryption key. Must be called before any other statement.
func (d *DB) Key(password string) error {
	cpw := C.CString(password)
	defer C.free(unsafe.Pointer(cpw))
	if rc := C.sqlite3_key(d.ptr, unsafe.Pointer(cpw), C.int(len(password))); rc != rcOK {
		return errors.New(d.errmsg())
	}
	return nil
}

// Rekey changes the encryption key of the database.
func (d *DB) Rekey(password string) error {
	cpw := C.CString(password)
	defer C.free(unsafe.Pointer(cpw))
	if rc := C.sqlite3_rekey(d.ptr, unsafe.Pointer(cpw), C.int(len(password))); rc != rcOK {
		return errors.New(d.errmsg())
	}
	return nil
}

// Close releases the database handle.
func (d *DB) Close() {
	if d.ptr != nil {
		C.sqlite3_close(d.ptr)
		d.ptr = nil
	}
}

func (d *DB) errmsg() string {
	return C.GoString(C.sqlite3_errmsg(d.ptr))
}

// Exec runs a statement that returns no rows.
func (d *DB) Exec(sql string, args ...string) error {
	stmt, err := d.prepare(sql)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)

	if err := d.bind(stmt, args); err != nil {
		return err
	}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case rcDone:
			return nil
		case rcRow:
			continue
		default:
			return errors.New(d.errmsg())
		}
	}
}

// Query runs a statement and returns all rows. NULL cells are nil pointers.
func (d *DB) Query(sql string, args ...string) ([][]*string, error) {
	stmt, err := d.prepare(sql)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)

	if err := d.bind(stmt, args); err != nil {
		return nil, err
	}
	ncol := int(C.sqlite3_column_count(stmt))
	var rows [][]*string
	for {
		rc := C.sqlite3_step(stmt)
		if rc == rcDone {
			break
		}
		if rc != rcRow {
			return nil, errors.New(d.errmsg())
		}
		row := make([]*string, ncol)
		for c := 0; c < ncol; c++ {
			if C.sqlite3_column_type(stmt, C.int(c)) == colNull {
				continue
			}
			p := C.sqlite3_column_text(stmt, C.int(c))
			if p == nil {
				continue
			}
			s := C.GoString((*C.char)(unsafe.Pointer(p)))
			row[c] = &s
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (d *DB) prepare(sql string) (*C.sqlite3_stmt, error) {
	csql := C.CString(sql)
	defer C.free(unsafe.Pointer(csql))

	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(d.ptr, csql, -1, &stmt, nil); rc != rcOK {
		return nil, errors.New(d.errmsg())
	}
	return stmt, nil
}

func (d *DB) bind(stmt *C.sqlite3_stmt, args []string) error {
	for i, a := range args {
		ca := C.CString(a)
		rc := C.sv_bind_text(stmt, C.int(i+1), ca, C.int(len(a)))
		C.free(unsafe.Pointer(ca))
		if rc != rcOK {
			return errors.New(d.errmsg())
		}
	}
	return nil
}
