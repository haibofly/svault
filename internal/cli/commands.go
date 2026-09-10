package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"secret-manager/internal/session"
	"secret-manager/internal/vault"
)

type entry struct {
	name  string
	value string
}

// global options set by Main.
var (
	optNoCache bool
	optTTL     = session.DefaultTTL
)

func defaultVaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "vault.db"
	}
	return filepath.Join(home, ".svault", "vault.db")
}

// Main is the svault entry point. It returns the process exit code.
func Main(argv []string) int {
	vaultPath := defaultVaultPath()

	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "--vault":
			if i+1 >= len(argv) {
				fmt.Fprintln(os.Stderr, "error: --vault requires a value")
				return 2
			}
			vaultPath = argv[i+1]
			i += 2
			continue
		case strings.HasPrefix(a, "--vault="):
			vaultPath = strings.TrimPrefix(a, "--vault=")
			i++
			continue
		case a == "--no-cache":
			optNoCache = true
			i++
			continue
		case a == "--ttl":
			if i+1 >= len(argv) {
				fmt.Fprintln(os.Stderr, "error: --ttl requires a value (e.g. 15m)")
				return 2
			}
			d, err := time.ParseDuration(argv[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid --ttl %q: %v\n", argv[i+1], err)
				return 2
			}
			optTTL = d
			i += 2
			continue
		case strings.HasPrefix(a, "--ttl="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "--ttl="))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid --ttl: %v\n", err)
				return 2
			}
			optTTL = d
			i++
			continue
		}
		break
	}
	args := argv[i:]
	if len(args) == 0 {
		usage()
		return 2
	}

	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "init":
		err = cmdInit(vaultPath, rest)
	case "put":
		err = cmdPut(vaultPath, rest)
	case "get":
		err = cmdGet(vaultPath, rest)
	case "list":
		err = cmdList(vaultPath, rest)
	case "rm":
		err = cmdRm(vaultPath, rest)
	case "passwd":
		err = cmdPasswd(vaultPath, rest)
	case "import":
		err = cmdImport(vaultPath, rest)
	case "export":
		err = cmdExport(vaultPath, rest)
	case "unlock":
		err = cmdUnlock(vaultPath, rest)
	case "lock":
		err = cmdLock(vaultPath, rest)
	case "status":
		err = cmdStatus(vaultPath, rest)
	case "-h", "--help", "help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n", cmd)
		usage()
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}
	return 0
}

func usage() {
	fmt.Fprint(os.Stderr, `svault - minimal local secret manager backed by SQLCipher

usage: svault [options] <command> [args]

options:
  --vault <path>     vault file (default: ~/.svault/vault.db)
  --no-cache         do not read or write the session password cache
  --ttl <duration>   session cache lifetime (default: 15m)

commands:
  init                         create a new vault
  put <name> [value]           add or update a secret
  get <name>                   print a secret value
  list                         list secret names
  rm <name>                    delete a secret
  passwd                       change the master password
  import [--encrypted] <file>  import entries from a JSON file (upsert)
  export [--encrypt] [--force] <file>
                               export all entries (JSON, or encrypted backup)
  unlock [--ttl <duration>]    enter the master password once and cache it
  lock                         clear the session password cache
  status                       show whether the vault is unlocked
`)
}

// --- password helpers -------------------------------------------------------

func askPassword(prompt string) (string, error) {
	s, err := readSecret(prompt)
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return s, nil
}

func askMaster() (string, error) {
	s, err := askPassword("Master password: ")
	if err != nil {
		return "", err
	}
	if s == "" {
		return "", fmt.Errorf("empty password, aborted")
	}
	return s, nil
}

func askNew(label string) (string, error) {
	a, err := askPassword(label + ": ")
	if err != nil {
		return "", err
	}
	if a == "" {
		return "", fmt.Errorf("empty password, aborted")
	}
	b, err := askPassword("Confirm " + strings.ToLower(label) + ": ")
	if err != nil {
		return "", err
	}
	if a != b {
		return "", fmt.Errorf("passwords do not match")
	}
	return a, nil
}

var errWrongMaster = fmt.Errorf("wrong master password (or corrupted vault)")

// getMasterPassword resolves the master password from (in order):
// the SVAULT_PASSWORD environment variable, the session cache, or a prompt.
func getMasterPassword(vaultPath string) (pw string, fromCache bool, err error) {
	if env := os.Getenv("SVAULT_PASSWORD"); env != "" {
		return env, false, nil
	}
	if !optNoCache {
		if cached, ok := session.Get(vaultPath); ok {
			return cached, true, nil
		}
	}
	pw, err = askMaster()
	return pw, false, err
}

// tryOpen opens a vault and validates the password.
func tryOpen(path, password string) (*vault.Vault, error) {
	v, err := vault.Open(path, password)
	if err != nil {
		return nil, err
	}
	if err := v.Validate(); err != nil {
		v.Close()
		return nil, err
	}
	return v, nil
}

func openVault(path string) (*vault.Vault, error) {
	if !exists(path) {
		return nil, fmt.Errorf("vault not found: %s\nrun `svault init` first", path)
	}

	pw, fromCache, err := getMasterPassword(path)
	if err != nil {
		return nil, err
	}

	v, err := tryOpen(path, pw)
	if err != nil && fromCache {
		// cached password is stale (e.g. the vault was rekeyed elsewhere)
		_ = session.Delete(path)
		fromCache = false
		if pw, err = askMaster(); err != nil {
			return nil, err
		}
		v, err = tryOpen(path, pw)
	}
	if err != nil {
		return nil, errWrongMaster
	}

	if shouldCache(fromCache) {
		_ = session.Put(path, pw, optTTL)
	}
	return v, nil
}

// shouldCache reports whether a freshly entered password should be cached.
func shouldCache(fromCache bool) bool {
	return !fromCache &&
		!optNoCache &&
		os.Getenv("SVAULT_PASSWORD") == "" &&
		stdinIsConsole()
}

// --- commands ---------------------------------------------------------------

func cmdInit(path string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: svault init")
	}
	if exists(path) {
		return fmt.Errorf("vault already exists: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(absPath(path)), 0o700); err != nil {
		return err
	}
	pw, err := askNew("Master password")
	if err != nil {
		return err
	}
	v, err := vault.Open(path, pw)
	if err != nil {
		return err
	}
	defer v.Close()
	if err := v.Init(); err != nil {
		return err
	}
	if shouldCache(false) {
		_ = session.Put(path, pw, optTTL)
	}
	fmt.Printf("initialized vault: %s\n", path)
	return nil
}

func cmdPut(path string, args []string) error {
	rest, err := parseNoFlags("put", args)
	if err != nil {
		return err
	}
	if len(rest) < 1 || len(rest) > 2 {
		return fmt.Errorf("usage: svault put <name> [value]")
	}
	name := rest[0]

	v, err := openVault(path)
	if err != nil {
		return err
	}
	defer v.Close()

	var value string
	if len(rest) == 2 {
		value = rest[1]
	} else {
		value, err = askPassword("Secret value: ")
		if err != nil {
			return err
		}
	}
	if err := v.Put(name, value); err != nil {
		return err
	}
	fmt.Printf("saved: %s\n", name)
	return nil
}

func cmdGet(path string, args []string) error {
	rest, err := parseNoFlags("get", args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: svault get <name>")
	}
	v, err := openVault(path)
	if err != nil {
		return err
	}
	defer v.Close()

	value, ok, err := v.Get(rest[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("not found: %s", rest[0])
	}
	fmt.Println(value)
	return nil
}

func cmdList(path string, args []string) error {
	rest, err := parseNoFlags("list", args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return fmt.Errorf("usage: svault list")
	}
	v, err := openVault(path)
	if err != nil {
		return err
	}
	defer v.Close()

	rows, err := v.List()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("(empty)")
		return nil
	}
	for _, r := range rows {
		fmt.Printf("%s\t%s\n", r[0], r[1])
	}
	return nil
}

func cmdRm(path string, args []string) error {
	rest, err := parseNoFlags("rm", args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: svault rm <name>")
	}
	v, err := openVault(path)
	if err != nil {
		return err
	}
	defer v.Close()

	if err := v.Delete(rest[0]); err != nil {
		return err
	}
	fmt.Printf("deleted: %s\n", rest[0])
	return nil
}

func cmdPasswd(path string, args []string) error {
	rest, err := parseNoFlags("passwd", args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return fmt.Errorf("usage: svault passwd")
	}
	v, err := openVault(path)
	if err != nil {
		return err
	}
	defer v.Close()

	newPw, err := askNew("Master password")
	if err != nil {
		return err
	}
	if err := v.Rekey(newPw); err != nil {
		return err
	}
	if !optNoCache && stdinIsConsole() {
		_ = session.Put(path, newPw, optTTL)
	} else {
		_ = session.Delete(path)
	}
	fmt.Println("master password changed")
	return nil
}

func cmdUnlock(path string, args []string) error {
	fs := flag.NewFlagSet("unlock", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ttlStr := fs.String("ttl", "", "session lifetime (e.g. 30m)")
	rest, err := parseFlagsFS(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return fmt.Errorf("usage: svault unlock [--ttl <duration>]")
	}
	ttl := optTTL
	if *ttlStr != "" {
		if ttl, err = time.ParseDuration(*ttlStr); err != nil {
			return fmt.Errorf("invalid --ttl: %w", err)
		}
	}
	if !exists(path) {
		return fmt.Errorf("vault not found: %s\nrun `svault init` first", path)
	}
	pw, err := askMaster()
	if err != nil {
		return err
	}
	v, err := tryOpen(path, pw)
	if err != nil {
		return errWrongMaster
	}
	v.Close()
	if err := session.Put(path, pw, ttl); err != nil {
		return fmt.Errorf("unlocked, but failed to cache the password: %w", err)
	}
	fmt.Printf("unlocked: %s (expires in %s)\n", path, ttl)
	return nil
}

func cmdLock(path string, args []string) error {
	rest, err := parseNoFlags("lock", args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return fmt.Errorf("usage: svault lock")
	}
	if err := session.Clear(); err != nil {
		return err
	}
	fmt.Println("locked (session password cache cleared)")
	return nil
}

func cmdStatus(path string, args []string) error {
	rest, err := parseNoFlags("status", args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return fmt.Errorf("usage: svault status")
	}
	if os.Getenv("SVAULT_PASSWORD") != "" {
		fmt.Println("unlocked (via SVAULT_PASSWORD environment variable)")
		return nil
	}
	if rem, ok := session.Status(path); ok {
		fmt.Printf("unlocked: %s (expires in %s)\n", path, rem.Round(time.Second))
		return nil
	}
	fmt.Printf("locked: %s\n", path)
	return nil
}

func cmdImport(path string, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	encrypted := fs.Bool("encrypted", false, "treat the file as an encrypted backup")
	rest, err := parseFlagsFS(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: svault import [--encrypted] <file>")
	}
	file := rest[0]

	var entries []entry
	if *encrypted {
		var err error
		entries, err = loadEncryptedEntries(file)
		if err != nil {
			return err
		}
	} else {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("file not found: %s", file)
		}
		var raw interface{}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&raw); err != nil {
			// not plaintext JSON -> assume encrypted backup
			entries, err = loadEncryptedEntries(file)
			if err != nil {
				return err
			}
		} else {
			entries, err = entriesFromRaw(raw)
			if err != nil {
				return err
			}
		}
	}

	v, err := openVault(path)
	if err != nil {
		return err
	}
	defer v.Close()

	existing, err := v.Names()
	if err != nil {
		return err
	}
	if err := v.Begin(); err != nil {
		return err
	}
	for _, e := range entries {
		if err := v.Put(e.name, e.value); err != nil {
			v.Rollback()
			return err
		}
	}
	if err := v.Commit(); err != nil {
		return err
	}

	added := 0
	for _, e := range entries {
		if !existing[e.name] {
			added++
		}
	}
	fmt.Printf("imported %d entries (%d new, %d updated) from %s\n", len(entries), added, len(entries)-added, file)
	return nil
}

func cmdExport(path string, args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	encrypt := fs.Bool("encrypt", false, "write an encrypted backup")
	force := fs.Bool("force", false, "overwrite an existing output file")
	rest, err := parseFlagsFS(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: svault export [--encrypt] [--force] <file>")
	}
	file := rest[0]

	v, err := openVault(path)
	if err != nil {
		return err
	}
	rows, err := v.ExportRows()
	v.Close()
	if err != nil {
		return err
	}

	if *encrypt {
		return exportEncrypted(file, rows, *force)
	}

	type rec struct {
		Name      string `json:"name"`
		Value     string `json:"value"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}
	recs := make([]rec, 0, len(rows))
	for _, r := range rows {
		recs = append(recs, rec{r[0], r[1], r[2], r[3]})
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if file == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	if exists(file) && !*force {
		return fmt.Errorf("refusing to overwrite existing file: %s (use --force)", file)
	}
	if err := os.MkdirAll(filepath.Dir(absPath(file)), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "exported %d entries to %s\n", len(recs), file)
	fmt.Fprintln(os.Stderr, "warning: exported file is PLAINTEXT - store and delete it securely")
	return nil
}

func exportEncrypted(file string, rows [][4]string, force bool) error {
	if file == "-" {
		return fmt.Errorf("--encrypt needs a file path (cannot write to stdout)")
	}
	if exists(file) && !force {
		return fmt.Errorf("refusing to overwrite existing file: %s (use --force)", file)
	}
	backupPw, err := askNew("Backup password")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absPath(file)), 0o700); err != nil {
		return err
	}
	bv, err := vault.Open(file, backupPw)
	if err != nil {
		return err
	}
	defer bv.Close()
	if err := bv.Init(); err != nil {
		return err
	}
	if err := bv.Begin(); err != nil {
		return err
	}
	for _, r := range rows {
		if err := bv.Exec("INSERT INTO secrets(name,value,created_at,updated_at) VALUES(?,?,?,?)", r[0], r[1], r[2], r[3]); err != nil {
			bv.Rollback()
			return err
		}
	}
	if err := bv.Commit(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "exported %d entries (encrypted) to %s\n", len(rows), file)
	return nil
}

// --- import helpers ---------------------------------------------------------

func loadEncryptedEntries(path string) ([]entry, error) {
	if !exists(path) {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	pw, err := askPassword("Backup password: ")
	if err != nil {
		return nil, err
	}
	v, err := vault.Open(path, pw)
	if err != nil {
		return nil, err
	}
	defer v.Close()

	rows, err := v.Query("SELECT name, value FROM secrets ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("wrong backup password (or not an encrypted backup)")
	}
	out := make([]entry, 0, len(rows))
	for _, r := range rows {
		out = append(out, entry{deref(r[0]), deref(r[1])})
	}
	return out, nil
}

func entriesFromRaw(raw interface{}) ([]entry, error) {
	switch v := raw.(type) {
	case map[string]interface{}:
		out := make([]entry, 0, len(v))
		for k, val := range v {
			s, err := scalarString(val)
			if err != nil {
				return nil, fmt.Errorf("entry %q: %w", k, err)
			}
			out = append(out, entry{k, s})
		}
		return out, nil
	case []interface{}:
		out := make([]entry, 0, len(v))
		for i, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("entry #%d is not an object", i+1)
			}
			nameRaw, ok := m["name"]
			name, ok2 := nameRaw.(string)
			if !ok || !ok2 || name == "" {
				return nil, fmt.Errorf("entry #%d: missing or invalid \"name\"", i+1)
			}
			valRaw, ok := m["value"]
			if !ok || valRaw == nil {
				return nil, fmt.Errorf("entry #%d (%s): missing \"value\"", i+1, name)
			}
			s, err := scalarString(valRaw)
			if err != nil {
				return nil, fmt.Errorf("entry #%d (%s): %w", i+1, name, err)
			}
			out = append(out, entry{name, s})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("JSON must be an array of {\"name\",\"value\"} objects or an object of name -> value")
	}
}

func scalarString(v interface{}) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("\"value\" must be a string")
	}
}

// --- small helpers ----------------------------------------------------------

func parseNoFlags(name string, args []string) ([]string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return fs.Args(), nil
}

// parseFlagsFS parses flags that may appear before or after positional args
// (the standard flag package stops at the first positional argument).
func parseFlagsFS(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, positional []string
	for _, a := range args {
		if a == "-" || !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
		} else {
			flags = append(flags, a)
		}
	}
	if err := fs.Parse(append(flags, positional...)); err != nil {
		return nil, err
	}
	return fs.Args(), nil
}

func absPath(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
