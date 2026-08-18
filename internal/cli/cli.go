// Package cli implements the administrative command-line interface bundled
// into the gourl binary. Commands run locally inside the container only —
// they never touch the HTTP API and need no authentication. The server
// starts when the binary runs without a subcommand.
//
// Sensitive operations (every reset target, webui on/off, restart) prompt
// for confirmation on a terminal; -y / --yes skips the prompt for scripts.
package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // driver name "sqlite", shared with internal/store
	"gopkg.in/yaml.v3"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/counter"
	"github.com/wmy2981/gourl/internal/logx"
	"github.com/wmy2981/gourl/internal/store"
	"github.com/wmy2981/gourl/internal/version"
)

// Main runs the CLI and returns the process exit code (0 ok, 1 error,
// 2 usage).
func Main(args []string) int {
	if len(args) == 0 {
		printHelp()
		return 2
	}
	// --help / -h anywhere prints the full help.
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printHelp()
			return 0
		}
	}
	switch args[0] {
	case "help":
		printHelp()
		return 0
	case "version":
		fmt.Println(version.Version)
		return 0
	case "status":
		return cmdStatus()
	case "health":
		return cmdHealth()
	case "config":
		return cmdConfig(args[1:])
	case "setup-code":
		return cmdSetupCode()
	case "log":
		return cmdLog(args[1:])
	case "db":
		return cmdDb(args[1:])
	case "reset":
		return cmdReset(args[1:])
	case "webui":
		return cmdWebUI(args[1:])
	case "restart":
		return cmdRestart(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q — run `gourl help`\n", args[0])
		return 2
	}
}

/* ---------- shared helpers ---------- */

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func cfgPath() string { return envOr("CONFIG_PATH", "./config/config.yaml") }
func dbPath() string  { return envOr("DB_PATH", "./data/gourl.db") }
func redisAddr() string {
	return envOr("REDIS_ADDR", "localhost:6379")
}

// confirm asks for interactive confirmation on a terminal; yes skips the
// prompt. Without a terminal and without -y the operation is refused.
func confirm(yes bool, prompt string) error {
	if yes {
		return nil
	}
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return errors.New("standard input is not a terminal — pass -y to proceed without confirmation")
	}
	fmt.Printf("%s [y/N]: ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	}
	return errors.New("aborted")
}

// splitYes separates -y / --yes from positional arguments.
func splitYes(args []string) (yes bool, rest []string) {
	for _, a := range args {
		if a == "-y" || a == "--yes" {
			yes = true
		} else {
			rest = append(rest, a)
		}
	}
	return yes, rest
}

// restartGourl stops the gourl process so the container's restart policy
// starts it again. Inside the container gourl is PID 1 (the entrypoint execs
// it); elsewhere this fails with a hint to restart manually.
func restartGourl() error {
	cmdline, err := os.ReadFile("/proc/1/cmdline")
	if err != nil {
		return fmt.Errorf("cannot find the gourl process (works inside the container): %w", err)
	}
	if !strings.Contains(strings.TrimRight(string(cmdline), "\x00"), "gourl") {
		return errors.New("PID 1 is not gourl — restart the service manually")
	}
	p, err := os.FindProcess(1)
	if err != nil {
		return err
	}
	return p.Signal(os.Interrupt)
}

// restartGourlFn is indirection over restartGourl so tests can observe the
// restart without a container.
var restartGourlFn = restartGourl

func loadConfig() (*config.Manager, error) {
	return config.NewManager(cfgPath())
}

// checkSQLite opens the database read-only and runs a trivial query.
func checkSQLite(path string) error {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var one int
	return db.QueryRow("SELECT 1").Scan(&one)
}

/* ---------- status / health ---------- */

func cmdStatus() int {
	fmt.Printf("gourl %s\n", version.Version)
	m, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
	} else {
		cur := m.Get()
		auth := "setup mode (no admin password)"
		if cur.PasswordHash != "" {
			auth = "admin password set"
		}
		webui := "disabled"
		if cur.WebUIEnabled {
			webui = "enabled"
		}
		fmt.Printf("config:  %s (%s, webui %s, log level %s)\n", cfgPath(), auth, webui, cur.LogLevel)
	}
	switch _, err := os.Stat(dbPath()); {
	case err != nil:
		fmt.Printf("database: %s (missing)\n", dbPath())
	case checkSQLite(dbPath()) != nil:
		fmt.Printf("database: %s (unreadable)\n", dbPath())
	default:
		fmt.Printf("database: %s (ok)\n", dbPath())
	}
	ctr := counter.New(redisAddr())
	defer ctr.Close()
	redisInfo := "unreachable"
	if err := ctr.Ping(context.Background()); err == nil {
		redisInfo = "ok"
	}
	fmt.Printf("redis:   %s (%s)\n", redisAddr(), redisInfo)
	return 0
}

func cmdHealth() int {
	code := 0
	if _, err := os.Stat(dbPath()); err != nil {
		fmt.Printf("sqlite: missing (%s)\n", dbPath())
		code = 1
	} else if err := checkSQLite(dbPath()); err != nil {
		fmt.Printf("sqlite: error: %v\n", err)
		code = 1
	} else {
		fmt.Println("sqlite: ok")
	}
	ctr := counter.New(redisAddr())
	defer ctr.Close()
	if err := ctr.Ping(context.Background()); err != nil {
		fmt.Printf("redis: error: %v\n", err)
		code = 1
	} else {
		fmt.Println("redis: ok")
	}
	return code
}

/* ---------- setup-code ---------- */

// cmdSetupCode prints the bootstrap code the running server persisted when
// it started in setup mode (no admin password yet). The file lives next to
// the database and is removed once the setup completes, so an exit code 1
// means the server is not in setup mode.
func cmdSetupCode() int {
	p := filepath.Join(filepath.Dir(dbPath()), "setup.code")
	data, err := os.ReadFile(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no setup code available: the server is not in setup mode (or the code file is missing)")
		return 1
	}
	fmt.Print(string(data))
	return 0
}

/* ---------- config show / log ---------- */

func cmdConfig(args []string) int {
	if len(args) != 1 || args[0] != "show" {
		fmt.Fprintln(os.Stderr, "usage: gourl config show")
		return 2
	}
	m, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	cur := m.Get()
	cur.PasswordHash = "" // never print credentials
	out, err := yaml.Marshal(cur)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal config: %v\n", err)
		return 1
	}
	fmt.Print(string(out))
	return 0
}

func cmdLog(args []string) int {
	lines := 100
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n <= 0 {
			fmt.Fprintln(os.Stderr, "usage: gourl log [lines]")
			return 2
		}
		lines = min(n, 1000)
	}
	data, err := os.ReadFile(logx.FileName())
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", logx.FileName(), err)
		return 1
	}
	ls := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	start := 0
	if len(ls) > lines {
		start = len(ls) - lines
	}
	for _, l := range ls[start:] {
		if l != "" {
			fmt.Println(l)
		}
	}
	return 0
}

/* ---------- db export ---------- */

// Row shapes mirror scripts/db-export.mts exactly: nullable columns stay
// null in JSON, deleted flags become booleans, backup ids become "b-N".
type exportLink struct {
	ID          int64   `json:"id"`
	Code        string  `json:"code"`
	URL         string  `json:"url"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	ExpiresAt   int64   `json:"expires_at"`
	ClickCount  int64   `json:"click_count"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
	Deleted     bool    `json:"deleted"`
}

type exportToken struct {
	ID        int64  `json:"id"`
	Token     string `json:"token"`
	Note      string `json:"note"`
	CreatedAt int64  `json:"created_at"`
	Deleted   bool   `json:"deleted"`
}

type exportDailyClick struct {
	LinkID *int64 `json:"link_id"`
	Code   string `json:"code"`
	Date   string `json:"date"`
	Count  int64  `json:"count"`
}

type exportBackup struct {
	BID         string  `json:"b_id"`
	LinkID      int64   `json:"link_id"`
	Code        string  `json:"code"`
	URL         string  `json:"url"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	ExpiresAt   int64   `json:"expires_at"`
	ClickCount  int64   `json:"click_count"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
	BackedAt    int64   `json:"backed_at"`
}

func cmdDb(args []string) int {
	if len(args) == 0 || args[0] != "export" {
		fmt.Fprintln(os.Stderr, "usage: gourl db export [out-dir]")
		return 2
	}
	outDir := "."
	if len(args) > 1 {
		outDir = args[1]
	}
	if _, err := os.Stat(dbPath()); err != nil {
		fmt.Fprintf(os.Stderr, "database not found: %s\n", dbPath())
		return 1
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output dir: %v\n", err)
		return 1
	}
	db, err := sql.Open("sqlite", "file:"+dbPath()+"?mode=ro")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		return 1
	}
	defer db.Close()
	nLinks, nTokens, nDaily, nBackups, err := exportDB(db, outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "export: %v\n", err)
		return 1
	}
	fmt.Printf("wrote %d links, %d tokens, %d daily-click rows, %d backups to %s\n",
		nLinks, nTokens, nDaily, nBackups, outDir)
	if nTokens > 0 {
		fmt.Fprintln(os.Stderr, "note: tokens.json contains full API token values — keep it private")
	}
	return 0
}

func exportDB(db *sql.DB, outDir string) (nLinks, nTokens, nDaily, nBackups int, err error) {
	links := []exportLink{}
	rows, err := db.Query(`SELECT id, code, url, title, description, expires_at, click_count, created_at, updated_at, deleted
		FROM links ORDER BY created_at DESC, rowid DESC`)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for rows.Next() {
		var l exportLink
		var deleted int
		if err := rows.Scan(&l.ID, &l.Code, &l.URL, &l.Title, &l.Description, &l.ExpiresAt,
			&l.ClickCount, &l.CreatedAt, &l.UpdatedAt, &deleted); err != nil {
			rows.Close()
			return 0, 0, 0, 0, err
		}
		l.Deleted = deleted != 0
		links = append(links, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, 0, 0, err
	}
	nLinks = len(links)

	tokens := []exportToken{}
	rows, err = db.Query(`SELECT id, token, note, created_at, deleted FROM api_tokens ORDER BY id DESC`)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for rows.Next() {
		var t exportToken
		var deleted int
		if err := rows.Scan(&t.ID, &t.Token, &t.Note, &t.CreatedAt, &deleted); err != nil {
			rows.Close()
			return 0, 0, 0, 0, err
		}
		t.Deleted = deleted != 0
		tokens = append(tokens, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, 0, 0, err
	}
	nTokens = len(tokens)

	dailies := []exportDailyClick{}
	rows, err = db.Query(`SELECT link_id, code, date, count FROM daily_clicks ORDER BY date DESC, code`)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for rows.Next() {
		var d exportDailyClick
		if err := rows.Scan(&d.LinkID, &d.Code, &d.Date, &d.Count); err != nil {
			rows.Close()
			return 0, 0, 0, 0, err
		}
		dailies = append(dailies, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, 0, 0, err
	}
	nDaily = len(dailies)

	backups := []exportBackup{}
	rows, err = db.Query(`SELECT b_id, link_id, code, url, title, description, expires_at, click_count, created_at, updated_at, backed_at
		FROM backups ORDER BY b_id`)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for rows.Next() {
		var b exportBackup
		var bID int64
		if err := rows.Scan(&bID, &b.LinkID, &b.Code, &b.URL, &b.Title, &b.Description, &b.ExpiresAt,
			&b.ClickCount, &b.CreatedAt, &b.UpdatedAt, &b.BackedAt); err != nil {
			rows.Close()
			return 0, 0, 0, 0, err
		}
		b.BID = fmt.Sprintf("b-%d", bID)
		backups = append(backups, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, 0, 0, err
	}
	nBackups = len(backups)

	if err := writeJSONFile(filepath.Join(outDir, "links.json"), links); err != nil {
		return 0, 0, 0, 0, err
	}
	if err := writeJSONFile(filepath.Join(outDir, "tokens.json"), tokens); err != nil {
		return 0, 0, 0, 0, err
	}
	if err := writeJSONFile(filepath.Join(outDir, "daily-clicks.json"), dailies); err != nil {
		return 0, 0, 0, 0, err
	}
	if err := writeJSONFile(filepath.Join(outDir, "backups.json"), backups); err != nil {
		return 0, 0, 0, 0, err
	}
	return nLinks, nTokens, nDaily, nBackups, nil
}

func writeJSONFile(path string, rows any) error {
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

/* ---------- reset ---------- */

func cmdReset(args []string) int {
	yes, targets := splitYes(args)
	if len(targets) == 0 {
		printResetHelp()
		return 2
	}
	switch targets[0] {
	case "password":
		return resetPassword(yes)
	case "uablock":
		return resetConfigField(yes, "clear all blocked user-agent patterns?", func(cur *config.Config) { cur.UABlocks = []string{} })
	case "ipblock":
		return resetConfigField(yes, "clear all banned IP rules?", func(cur *config.Config) { cur.IPBlocks = []string{} })
	case "sessions":
		return resetConfigField(yes, "revoke every admin session?", func(cur *config.Config) { cur.SessionEpoch++ })
	case "config":
		return resetConfigFile(yes)
	case "api":
		return resetTokens(yes)
	case "db":
		return resetDB(yes)
	case "redis":
		return resetRedis(yes)
	case "--all":
		return resetAll(yes)
	default:
		fmt.Fprintf(os.Stderr, "unknown reset target %q — run `gourl help`\n", targets[0])
		return 2
	}
}

// resetConfigField mutates the live config after confirmation; the running
// server picks the change up without a restart.
func resetConfigField(yes bool, prompt string, mutate func(*config.Config)) int {
	if err := confirm(yes, prompt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	m, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	cur := m.Get()
	mutate(cur)
	if err := m.Update(cur); err != nil {
		fmt.Fprintf(os.Stderr, "update config: %v\n", err)
		return 1
	}
	fmt.Println("done")
	return 0
}

// resetPassword clears the admin password and restarts the service: the
// container comes back in setup mode with a fresh bootstrap code.
func resetPassword(yes bool) int {
	if err := confirm(yes, "clear the admin password and restart the service? the server will start in setup mode"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	m, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	cur := m.Get()
	cur.PasswordHash = ""
	if err := m.Update(cur); err != nil {
		fmt.Fprintf(os.Stderr, "update config: %v\n", err)
		return 1
	}
	fmt.Println("admin password cleared; restarting the service now")
	if err := restartGourlFn(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func resetConfigFile(yes bool) int {
	if err := confirm(yes, "delete the config file and restart the service? the server starts with defaults"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.Remove(cfgPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "remove config: %v\n", err)
		return 1
	}
	fmt.Println("config file removed; restarting the service now")
	if err := restartGourlFn(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func resetTokens(yes bool) int {
	if err := confirm(yes, "revoke every API token?"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := os.Stat(dbPath()); err != nil {
		fmt.Fprintln(os.Stderr, "no database, nothing to revoke")
		return 1
	}
	st, err := store.Open(dbPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		return 1
	}
	defer st.Close()
	n, err := st.RevokeAllTokens(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "revoke tokens: %v\n", err)
		return 1
	}
	fmt.Printf("revoked %d token(s)\n", n)
	return 0
}

func resetDB(yes bool) int {
	if err := confirm(yes, "delete the database and restart the service? click history is deleted with it"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, p := range []string{dbPath(), dbPath() + "-wal", dbPath() + "-shm"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "remove %s: %v\n", p, err)
			return 1
		}
	}
	fmt.Println("database deleted; restarting the service now")
	if err := restartGourlFn(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func resetRedis(yes bool) int {
	if err := confirm(yes, "wipe the Redis click buffer and restart the service?"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctr := counter.New(redisAddr())
	defer ctr.Close()
	if err := ctr.Ping(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "redis unreachable: %v\n", err)
		return 1
	}
	if err := ctr.FlushAll(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "flush redis: %v\n", err)
		return 1
	}
	fmt.Println("redis wiped; restarting the service now")
	if err := restartGourlFn(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func resetAll(yes bool) int {
	dataDir := filepath.Dir(dbPath())
	confDir := filepath.Dir(cfgPath())
	if err := confirm(yes, fmt.Sprintf("delete %s and %s and restart the service? ALL data is lost", confDir, dataDir)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, dir := range []string{dataDir, confDir} {
		if dir == "." || dir == "" {
			continue // never remove the working directory
		}
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "remove %s: %v\n", dir, err)
			return 1
		}
	}
	fmt.Println("data and config directories deleted; restarting the service now")
	if err := restartGourlFn(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

/* ---------- webui / restart ---------- */

func cmdWebUI(args []string) int {
	yes, rest := splitYes(args)
	if len(rest) != 1 || (rest[0] != "on" && rest[0] != "off") {
		fmt.Fprintln(os.Stderr, "usage: gourl webui on|off [-y]")
		return 2
	}
	enable := rest[0] == "on"
	if err := confirm(yes, fmt.Sprintf("turn the admin console %s?", rest[0])); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	m, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	cur := m.Get()
	cur.WebUIEnabled = enable
	if err := m.Update(cur); err != nil {
		fmt.Fprintf(os.Stderr, "update config: %v\n", err)
		return 1
	}
	fmt.Printf("admin console %s (takes effect immediately)\n", rest[0])
	return 0
}

func cmdRestart(args []string) int {
	yes, rest := splitYes(args)
	if len(rest) != 0 {
		fmt.Fprintln(os.Stderr, "usage: gourl restart [-y]")
		return 2
	}
	if err := confirm(yes, "restart the service?"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("restarting the service now")
	if err := restartGourlFn(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
