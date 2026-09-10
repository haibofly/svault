package vault

import (
	"secret-manager/internal/sqlcipher"
)

// Vault is a SQLCipher-backed secret store.
type Vault struct {
	db *sqlcipher.DB
}

// Open opens a vault with the given master password. It does not validate the
// password; call Validate to detect a wrong password.
func Open(path, password string) (*Vault, error) {
	db, err := sqlcipher.Open(path)
	if err != nil {
		return nil, err
	}
	if err := db.Key(password); err != nil {
		db.Close()
		return nil, err
	}
	return &Vault{db: db}, nil
}

// Close releases the vault.
func (v *Vault) Close() {
	v.db.Close()
}

// Validate triggers decryption so a wrong password surfaces as an error.
func (v *Vault) Validate() error {
	_, err := v.db.Query("SELECT count(*) FROM sqlite_master")
	return err
}

// Init creates the secrets table.
func (v *Vault) Init() error {
	return v.db.Exec(
		"CREATE TABLE secrets (" +
			"name TEXT PRIMARY KEY, " +
			"value TEXT NOT NULL, " +
			"created_at TEXT DEFAULT (datetime('now','localtime')), " +
			"updated_at TEXT DEFAULT (datetime('now','localtime')))")
}

// Put inserts or updates a secret.
func (v *Vault) Put(name, value string) error {
	return v.db.Exec(
		"INSERT INTO secrets(name, value) VALUES(?, ?) "+
			"ON CONFLICT(name) DO UPDATE SET value=excluded.value, updated_at=datetime('now','localtime')",
		name, value)
}

// Get returns the value for name and whether it exists.
func (v *Vault) Get(name string) (string, bool, error) {
	rows, err := v.db.Query("SELECT value FROM secrets WHERE name=?", name)
	if err != nil {
		return "", false, err
	}
	if len(rows) == 0 || rows[0][0] == nil {
		return "", false, nil
	}
	return *rows[0][0], true, nil
}

// List returns name/updated_at pairs ordered by name.
func (v *Vault) List() ([][2]string, error) {
	rows, err := v.db.Query("SELECT name, updated_at FROM secrets ORDER BY name")
	if err != nil {
		return nil, err
	}
	out := make([][2]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, [2]string{deref(r[0]), deref(r[1])})
	}
	return out, nil
}

// Names returns the set of existing secret names.
func (v *Vault) Names() (map[string]bool, error) {
	rows, err := v.db.Query("SELECT name FROM secrets")
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(rows))
	for _, r := range rows {
		set[deref(r[0])] = true
	}
	return set, nil
}

// ExportRows returns all rows as name,value,created_at,updated_at.
func (v *Vault) ExportRows() ([][4]string, error) {
	rows, err := v.db.Query("SELECT name, value, created_at, updated_at FROM secrets ORDER BY name")
	if err != nil {
		return nil, err
	}
	out := make([][4]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, [4]string{deref(r[0]), deref(r[1]), deref(r[2]), deref(r[3])})
	}
	return out, nil
}

// Delete removes a secret.
func (v *Vault) Delete(name string) error {
	return v.db.Exec("DELETE FROM secrets WHERE name=?", name)
}

// Rekey changes the master password.
func (v *Vault) Rekey(newPassword string) error {
	return v.db.Rekey(newPassword)
}

// Exec runs a raw statement.
func (v *Vault) Exec(sql string, args ...string) error {
	return v.db.Exec(sql, args...)
}

// Query runs a raw query.
func (v *Vault) Query(sql string, args ...string) ([][]*string, error) {
	return v.db.Query(sql, args...)
}

// Begin starts a transaction.
func (v *Vault) Begin() error { return v.db.Exec("BEGIN") }

// Commit commits the current transaction.
func (v *Vault) Commit() error { return v.db.Exec("COMMIT") }

// Rollback rolls back the current transaction.
func (v *Vault) Rollback() error { return v.db.Exec("ROLLBACK") }

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
