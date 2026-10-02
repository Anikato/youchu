# 基础登录与存储 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 做一个只能单账号登录的本机服务：Go 把会话放进 SQLite，Vite 提供中文登录页和登录后的占位页。

**Architecture:** `config.Load` 只读环境变量。嵌入迁移编号确认连续后才打开数据库。`modernc.org/sqlite` 的 DSN 在每条物理连接上打开外键、忙等和 WAL。HTTP 与迁移、账号、会话调用同一批 Go 函数。网页只通过 Vite 代理访问 `/api/v1`。

**Tech Stack:** Go 1.27、`modernc.org/sqlite` v1.60.1、`golang.org/x/crypto/argon2`、React 19.3.0、Vite 8.3.1、TypeScript 7.0.2、React Router 8.4.0、TanStack Query 5.104.0。

规格：`docs/superpowers/specs/2026-09-29-auth-storage-design.md`。

提交：本目录还不是 Git 仓库，也没有 `user.name`。每个提交步骤在执行前检查 `git rev-parse --is-inside-work-tree` 和 `git config user.name`。仓库不存在就先 `git init`。用户名为空就停下来问用户，不要改 Git 配置，也不要编造作者。

---

## 文件职责

- `internal/config/config.go`：环境变量解析。不打开数据库。
- `internal/store/store.go`：打开 SQLite，DSN 参数施加于每条新连接。
- `internal/migrate/migrate.go`：检查编号、创建 `schema_migrations`、只执行剩余前缀之后的迁移。
- `migrations/001_auth.sql`：`users` 与 `sessions`。
- `internal/auth/password.go`：argon2id PHC。
- `internal/auth/user.go`：空表时创建唯一用户。
- `internal/auth/session.go`：令牌只以 SHA-256 入库。
- `internal/httpapi/server.go`：登录、退出、当前用户、Origin、正文大小、限流。
- `internal/app/app.go`：按规格顺序组装，失败时不监听。
- `cmd/youchu/main.go`：信号处理和优雅关闭。
- `web/`：声明式 React Router 登录页与占位页。

模块路径是 `youchu`。

### Task 1: 模块与忽略规则

**Files:**
- Create: `.gitignore`
- Create: `go.mod`

- [ ] **Step 1: 写 `.gitignore`**

```gitignore
/data/
*.db
*.db-wal
*.db-shm
/web/node_modules/
/web/dist/
```

- [ ] **Step 2: 初始化模块**

Run: `go mod init youchu`

Expected: 创建 `go.mod`，module 为 `youchu`，go 版本为 `1.27`。

- [ ] **Step 3: 确认测试入口存在**

Run: `go test ./...`

Expected: 没有包，命令成功且没有业务测试。

- [ ] **Step 4: Commit**

```bash
git add .gitignore go.mod docs
git commit -m "chore: 初始化有处模块"
```

### Task 2: 配置校验

**Files:**
- Create: `internal/config/config_test.go`
- Create: `internal/config/config.go`

- [ ] **Step 1: 写失败测试**

```go
package config

import (
	"testing"
	"time"
)

func getenvFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"YOUCHU_PUBLIC_ORIGIN": "http://127.0.0.1:5173",
		"YOUCHU_USERNAME":      "ada",
		"YOUCHU_PASSWORD":      "correct-horse",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(getenvFrom(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" || cfg.DataDir != "./data" || cfg.CookieSecure {
		t.Fatalf("defaults: %+v", cfg)
	}
	if cfg.SessionTTL != 336*time.Hour || cfg.LoginLimit != 10 {
		t.Fatalf("ttl or limit: %+v", cfg)
	}
	if cfg.PublicOrigin != "http://127.0.0.1:5173" {
		t.Fatal(cfg.PublicOrigin)
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	cases := []map[string]string{
		{"YOUCHU_SESSION_TTL": "0s"},
		{"YOUCHU_SESSION_TTL": "-1s"},
		{"YOUCHU_SESSION_TTL": "500ms"},
		{"YOUCHU_LOGIN_LIMIT": "0"},
		{"YOUCHU_LOGIN_LIMIT": "-1"},
		{"YOUCHU_LOGIN_LIMIT": "1.5"},
		{"YOUCHU_COOKIE_SECURE": "yes"},
		{"YOUCHU_PUBLIC_ORIGIN": "http://127.0.0.1:5173/"},
		{"YOUCHU_PUBLIC_ORIGIN": "http://127.0.0.1:5173/app"},
		{},
	}
	for _, override := range cases {
		env := validEnv()
		for k, v := range override {
			env[k] = v
		}
		if _, ok := override["YOUCHU_PUBLIC_ORIGIN"]; !ok && len(override) == 0 {
			delete(env, "YOUCHU_PUBLIC_ORIGIN")
		}
		if _, err := Load(getenvFrom(env)); err == nil {
			t.Fatalf("accepted %v", override)
		}
	}
}

func TestValidateNewUser(t *testing.T) {
	name, err := ValidateNewUser("  ada  ", "correct-horse")
	if err != nil || name != "ada" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	if _, err := ValidateNewUser("a b", "correct-horse"); err == nil {
		t.Fatal("accepted internal space")
	}
	if _, err := ValidateNewUser("ada", "short"); err == nil {
		t.Fatal("accepted short password")
	}
	long := make([]rune, 129)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := ValidateNewUser("ada", string(long)); err == nil {
		t.Fatal("accepted long password")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/config`

Expected: FAIL，`Load` 未定义。

- [ ] **Step 3: 实现配置**

```go
package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Config struct {
	HTTPAddr     string
	DataDir      string
	PublicOrigin string
	CookieSecure bool
	SessionTTL   time.Duration
	LoginLimit   int
	Username     string
	Password     string
}

func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	cfg := Config{
		HTTPAddr:   "127.0.0.1:8080",
		DataDir:    "./data",
		SessionTTL: 336 * time.Hour,
		LoginLimit: 10,
	}
	if v := getenv("YOUCHU_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := getenv("YOUCHU_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	origin := getenv("YOUCHU_PUBLIC_ORIGIN")
	if err := validateOrigin(origin); err != nil {
		return Config{}, err
	}
	cfg.PublicOrigin = origin
	secure, err := parseBool(getenv("YOUCHU_COOKIE_SECURE"))
	if err != nil {
		return Config{}, fmt.Errorf("YOUCHU_COOKIE_SECURE: %w", err)
	}
	cfg.CookieSecure = secure
	if v := getenv("YOUCHU_SESSION_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Second {
			return Config{}, fmt.Errorf("YOUCHU_SESSION_TTL must be at least 1s")
		}
		cfg.SessionTTL = d
	}
	if v := getenv("YOUCHU_LOGIN_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("YOUCHU_LOGIN_LIMIT must be an integer >= 1")
		}
		cfg.LoginLimit = n
	}
	cfg.Username = getenv("YOUCHU_USERNAME")
	cfg.Password = getenv("YOUCHU_PASSWORD")
	return cfg, nil
}

func parseBool(v string) (bool, error) {
	switch v {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("must be true or false")
	}
}

func validateOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" || origin != u.Scheme+"://"+u.Host {
		return fmt.Errorf("YOUCHU_PUBLIC_ORIGIN must be scheme://host with no path, query, or trailing slash")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("YOUCHU_PUBLIC_ORIGIN scheme must be http or https")
	}
	return nil
}

func ValidateNewUser(username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || utf8.RuneCountInString(username) > 64 {
		return "", fmt.Errorf("invalid username")
	}
	for _, r := range username {
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("invalid username")
		}
	}
	if n := utf8.RuneCountInString(password); n < 8 || n > 128 {
		return "", fmt.Errorf("invalid password")
	}
	return username, nil
}
```

`validateOrigin` 用重新拼接 `scheme://host` 与原字符串比较，从而拒绝路径、查询、片段和末尾斜杠。用户名与密码按 Unicode 码点计数。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/config`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat: 校验登录服务的启动配置"
```

### Task 3: 每条连接都生效的 SQLite

**Files:**
- Create: `internal/store/store_test.go`
- Create: `internal/store/store.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: 写失败测试**

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAppliesPragmasOnEachConnection(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "youchu.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxIdleConns(0)
	if _, err := db.ExecContext(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY CHECK (id = 1))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE sessions (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id))`); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		var fk, busy int
		if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
			t.Fatalf("foreign_keys=%d err=%v", fk, err)
		}
		if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busy); err != nil || busy != 5000 {
			t.Fatalf("busy_timeout=%d err=%v", busy, err)
		}
	}
	check()
	check()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `INSERT INTO sessions(user_id) VALUES (9)`); err == nil {
		t.Fatal("foreign key was not enforced")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/store`

Expected: FAIL，`Open` 未定义。

- [ ] **Step 3: 安装驱动并实现打开函数**

Run: `go get modernc.org/sqlite@v1.60.1`

```go
package store

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	u := url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(abs),
		RawQuery: "_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL",
	}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	return db, nil
}
```

不要在 `Open` 里执行 `PRAGMA`。驱动会在每次新建物理连接时应用 DSN。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/store`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/store go.mod go.sum
git commit -m "feat: 为每个 SQLite 连接启用外键和忙等"
```

### Task 4: 迁移前缀

**Files:**
- Create: `internal/migrate/migrate_test.go`
- Create: `internal/migrate/migrate.go`
- Create: `migrations/001_auth.sql`
- Create: `migrations/embed.go`

- [ ] **Step 1: 写失败测试**

```go
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"youchu/internal/store"
)

func TestCheckEmbeddedContiguous(t *testing.T) {
	if err := CheckEmbedded([]File{{Version: 1, Name: "001_a.sql"}, {Version: 2, Name: "002_b.sql"}}); err != nil {
		t.Fatal(err)
	}
	if err := CheckEmbedded([]File{{Version: 1, Name: "001_a.sql"}, {Version: 3, Name: "003_c.sql"}}); err == nil {
		t.Fatal("accepted a gap")
	}
	if err := CheckEmbedded([]File{{Version: 1, Name: "001_a.sql"}, {Version: 1, Name: "001_b.sql"}}); err == nil {
		t.Fatal("accepted a duplicate")
	}
}

func TestStatementsKeepQuotedSemicolon(t *testing.T) {
	got := Statements("CREATE TABLE a(n TEXT); INSERT INTO a(n) VALUES ('a;b');")
	if len(got) != 2 || got[1] != "INSERT INTO a(n) VALUES ('a;b')" {
		t.Fatalf("%#v", got)
	}
}

func TestApplyTwiceKeepsOneRow(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	files := []File{{Version: 1, Name: "001_auth.sql", SQL: "CREATE TABLE marker(id INTEGER);"}}
	if err := Apply(ctx, db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = 1`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
}

func TestApplyRunsOnlyRemainder(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	if err := Apply(ctx, db, []File{{Version: 1, Name: "001_old.sql", SQL: "CREATE TABLE old(id INTEGER);"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	files := []File{
		{Version: 1, Name: "001_new.sql", SQL: "CREATE TABLE ignored(id INTEGER);"},
		{Version: 2, Name: "002_new.sql", SQL: "CREATE TABLE added(id INTEGER);"},
	}
	if err := Apply(ctx, db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM schema_migrations WHERE version = 1`).Scan(&name); err != nil || name != "001_old.sql" {
		t.Fatalf("name=%s err=%v", name, err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM added`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'ignored'`).Scan(&n); err != nil || n != 0 {
		t.Fatal("re-executed 001")
	}
}

func TestApplyRejectsNewerDatabase(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	files := []File{
		{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"},
		{Version: 2, Name: "002_b.sql", SQL: "SELECT 1;"},
	}
	if err := Apply(ctx, db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := db.QueryRowContext(ctx, `SELECT name FROM schema_migrations WHERE version = 2`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	err := Apply(ctx, db, []File{{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"}}, time.Now())
	if !errors.Is(err, ErrDatabaseNewer) {
		t.Fatalf("err=%v", err)
	}
	var after string
	if err := db.QueryRowContext(ctx, `SELECT name FROM schema_migrations WHERE version = 2`).Scan(&after); err != nil || after != before {
		t.Fatalf("row changed: %s -> %s err=%v", before, after, err)
	}
}

func TestApplyRejectsBrokenPrefix(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	if err := Apply(ctx, db, []File{{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, applied_at) VALUES (2, '002_x.sql', '2026-09-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	err := Apply(ctx, db, []File{
		{Version: 1, Name: "001_a.sql", SQL: "SELECT 1;"},
		{Version: 2, Name: "002_b.sql", SQL: "SELECT 1;"},
	}, time.Now())
	if !errors.Is(err, ErrAppliedPrefix) {
		t.Fatalf("err=%v", err)
	}
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "youchu.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/migrate`

Expected: FAIL，`CheckEmbedded` 未定义。

- [ ] **Step 3: 实现迁移器**

```go
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrDatabaseNewer = errors.New("database version is newer than this program")
var ErrAppliedPrefix = errors.New("applied migrations are not a contiguous prefix")

type File struct {
	Version int
	Name    string
	SQL     string
}

func CheckEmbedded(files []File) error {
	if len(files) == 0 {
		return fmt.Errorf("no migrations")
	}
	seen := append([]File(nil), files...)
	sort.Slice(seen, func(i, j int) bool { return seen[i].Version < seen[j].Version })
	for i, f := range seen {
		if f.Version != i+1 {
			return fmt.Errorf("embedded migrations must be contiguous from 001")
		}
	}
	return nil
}

func Statements(script string) []string {
	var b strings.Builder
	var out []string
	inSingle := false
	for i := 0; i < len(script); i++ {
		c := script[i]
		if c == '\'' {
			b.WriteByte(c)
			if inSingle && i+1 < len(script) && script[i+1] == '\'' {
				b.WriteByte('\'')
				i++
				continue
			}
			inSingle = !inSingle
			continue
		}
		if c == ';' && !inSingle {
			if s := strings.TrimSpace(b.String()); s != "" {
				out = append(out, s)
			}
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		out = append(out, s)
	}
	return out
}

func Apply(ctx context.Context, db *sql.DB, files []File, now time.Time) error {
	if err := CheckEmbedded(files); err != nil {
		return err
	}
	ordered := append([]File(nil), files...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Version < ordered[j].Version })
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var applied []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return err
		}
		applied = append(applied, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	embeddedMax := ordered[len(ordered)-1].Version
	appliedMax := 0
	for i, v := range applied {
		if v > embeddedMax {
			return ErrDatabaseNewer
		}
		if v != i+1 {
			return ErrAppliedPrefix
		}
		appliedMax = v
	}
	for _, f := range ordered {
		if f.Version <= appliedMax {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, stmt := range Statements(f.SQL) {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				tx.Rollback()
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)`, f.Version, f.Name, now.UTC().Format(time.RFC3339Nano)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func ParseVersion(name string) (int, error) {
	if len(name) < 8 || name[3] != '_' || !strings.HasSuffix(name, ".sql") {
		return 0, fmt.Errorf("bad migration name %s", name)
	}
	n, err := strconv.Atoi(name[:3])
	if err != nil || n < 1 || fmt.Sprintf("%03d", n) != name[:3] {
		return 0, fmt.Errorf("bad migration name %s", name)
	}
	return n, nil
}
```

- [ ] **Step 4: 写入生产迁移**

`migrations/001_auth.sql`：

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE sessions (
    id INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
```

`migrations/embed.go`：

```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

在 `internal/migrate/migrate_test.go` 增加一个测试：读取 `youchu/migrations` 的 `FS`，解析文件名，`Apply` 后 `users` 和 `sessions` 都存在。解析函数使用 `ParseVersion`。缺号测试只调用 `CheckEmbedded`，不调用 `store.Open`。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/migrate ./migrations`

Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/migrate migrations
git commit -m "feat: 按连续前缀执行 SQLite 迁移"
```

### Task 5: 密码哈希

**Files:**
- Create: `internal/auth/password_test.go`
- Create: `internal/auth/password.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: 写失败测试**

```go
package auth

import "testing"

func TestHashVerifiesOnlySamePassword(t *testing.T) {
	hash, err := Hash("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(hash, "correct-horse") {
		t.Fatal("same password rejected")
	}
	if Verify(hash, "wrong-password") {
		t.Fatal("wrong password accepted")
	}
	other, err := Hash("correct-horse")
	if err != nil || other == hash {
		t.Fatal("salt was not random")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/auth`

Expected: FAIL，`Hash` 未定义。

- [ ] **Step 3: 安装 argon2 并实现 PHC**

Run: `go get golang.org/x/crypto`

`go.sum` 会固定解析到的版本。`go.mod` 不写浮动版本。

```go
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

func Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=4$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum)), nil
}

func Verify(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func NewDummyHash() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return Hash(strconv.Itoa(int(buf[0])) + string(buf))
}
```

参数固定为 memory 65536 KiB、iterations 3、parallelism 4、salt 16 字节、输出 32 字节。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/auth`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/auth/password.go internal/auth/password_test.go go.mod go.sum
git commit -m "feat: 用 argon2id 保存密码"
```

### Task 6: 唯一账号

**Files:**
- Create: `internal/auth/user_test.go`
- Create: `internal/auth/user.go`

- [ ] **Step 1: 写失败测试**

```go
package auth

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"youchu/internal/migrate"
	"youchu/internal/store"
	"youchu/migrations"
)

func TestBootstrapCreatesOnlyOnce(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	if err := Bootstrap(ctx, db, "", ""); err == nil {
		t.Fatal("accepted empty credentials")
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
	if err := Bootstrap(ctx, db, "ada", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	var id int
	var hash string
	if err := db.QueryRowContext(ctx, `SELECT id, password_hash FROM users`).Scan(&id, &hash); err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("id=%d", id)
	}
	if err := Bootstrap(ctx, db, "ada", "another-password"); err != nil {
		t.Fatal(err)
	}
	var again string
	if err := db.QueryRowContext(ctx, `SELECT password_hash FROM users`).Scan(&again); err != nil || again != hash {
		t.Fatal("password hash changed")
	}
}

func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "youchu.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var files []migrate.File
	err = fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		version, err := migrate.ParseVersion(path)
		if err != nil {
			return err
		}
		body, err := fs.ReadFile(migrations.FS, path)
		if err != nil {
			return err
		}
		files = append(files, migrate.File{Version: version, Name: path, SQL: string(body)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	return db
}
```

`migratedDB` 与同包的会话测试共用。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/auth -run TestBootstrap`

Expected: FAIL，`Bootstrap` 未定义。

- [ ] **Step 3: 实现 Bootstrap**

```go
package auth

import (
	"context"
	"database/sql"
	"time"

	"youchu/internal/config"
)

func Bootstrap(ctx context.Context, db *sql.DB, username, password string) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	name, err := config.ValidateNewUser(username, password)
	if err != nil {
		return err
	}
	hash, err := Hash(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.ExecContext(ctx, `INSERT INTO users(id, username, password_hash, created_at, updated_at) VALUES (1, ?, ?, ?, ?)`, name, hash, now, now)
	return err
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/auth -run TestBootstrap`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/auth/user.go internal/auth/user_test.go
git commit -m "feat: 在空库创建唯一账号"
```

### Task 7: 会话令牌

**Files:**
- Create: `internal/auth/session_test.go`
- Create: `internal/auth/session.go`

- [ ] **Step 1: 写失败测试**

```go
package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionStoresHashAndExpires(t *testing.T) {
	ctx := context.Background()
	db := migratedDB(t)
	if err := Bootstrap(ctx, db, "ada", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	token, err := Create(ctx, db, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.QueryRowContext(ctx, `SELECT token_hash FROM sessions`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "" || stored == token {
		t.Fatal("stored the raw token")
	}
	name, err := Lookup(ctx, db, token, now.Add(time.Minute))
	if err != nil || name != "ada" {
		t.Fatalf("name=%s err=%v", name, err)
	}
	past := now.Add(-time.Second).Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `UPDATE sessions SET expires_at = ?`, past); err != nil {
		t.Fatal(err)
	}
	if _, err := Lookup(ctx, db, token, now); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows=%d err=%v", n, err)
	}
	first, err := Create(ctx, db, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Create(ctx, db, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(ctx, db, first); err != nil {
		t.Fatal(err)
	}
	if _, err := Lookup(ctx, db, first, now); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal(err)
	}
	if name, err := Lookup(ctx, db, second, now); err != nil || name != "ada" {
		t.Fatalf("second session name=%s err=%v", name, err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/auth -run 'TestSession'`

Expected: FAIL，`Create` 未定义。

- [ ] **Step 3: 实现会话**

```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

var ErrUnauthenticated = errors.New("unauthenticated")

func Create(ctx context.Context, db *sql.DB, ttl time.Duration, now time.Time) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	created := now.UTC()
	_, err := db.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at, created_at) VALUES (?, 1, ?, ?)`,
		tokenHash(token), created.Add(ttl).Format(time.RFC3339Nano), created.Format(time.RFC3339Nano))
	return token, err
}

func Lookup(ctx context.Context, db *sql.DB, token string, now time.Time) (string, error) {
	var username, expText string
	err := db.QueryRowContext(ctx, `SELECT users.username, sessions.expires_at FROM sessions JOIN users ON users.id = sessions.user_id WHERE sessions.token_hash = ?`, tokenHash(token)).Scan(&username, &expText)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUnauthenticated
	}
	if err != nil {
		return "", err
	}
	exp, err := time.Parse(time.RFC3339Nano, expText)
	if err != nil {
		return "", err
	}
	if !now.Before(exp) {
		_, _ = db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash(token))
		return "", ErrUnauthenticated
	}
	return username, nil
}

func Delete(ctx context.Context, db *sql.DB, token string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash(token))
	return err
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/auth -run 'TestSession'`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/auth/session.go internal/auth/session_test.go
git commit -m "feat: 将会话令牌的摘要存入 SQLite"
```

### Task 8: HTTP 接口

**Files:**
- Create: `internal/httpapi/server_test.go`
- Create: `internal/httpapi/server.go`

- [ ] **Step 1: 写失败测试**

准备迁移后的库、一个账号、`LoginLimit: 10`、`SessionTTL: time.Hour`、`PublicOrigin: "http://127.0.0.1:5173"`、`NewDummyHash()`。`New(db, cfg, dummyHash, now)` 返回 `http.Handler`。每个请求设置 `RemoteAddr` 和需要的 `Origin`。

断言：

- 正确密码返回 200、`{"username":"ada"}` 和 `youchu_session` Cookie。Cookie 有 `HttpOnly`、`SameSite=Lax`，`Secure` 跟随配置。随后带 Cookie 的 `GET /api/v1/me` 返回该用户名。
- 错误密码与不存在的用户名都返回 401、code `invalid_credentials`、message `用户名或密码错误`，且没有会话 Cookie。
- 无 Cookie 的 `GET /api/v1/me` 返回 401、code `unauthenticated`。
- 把会话 `expires_at` 改到过去后，`GET /api/v1/me` 返回 401。
- 缺少 Origin 或 Origin 不匹配的 `POST` 返回 403、code `origin_rejected`，`sessions` 行数仍为 0。
- 同一 `RemoteAddr` 第 11 次 `POST` 返回 429、code `rate_limited`。另一个 `RemoteAddr` 的第一次仍返回 401 或 200，而不是 429。
- `DELETE /api/v1/session` 带正确 Origin 后，原 Cookie 访问 `me` 返回 401；另一条会话仍有效。没有 Cookie 的 `DELETE` 也返回 204。
- 空用户名、空密码、非法 JSON 返回 400、code `invalid_body`。
- 超过 4096 字节的 `POST` 返回 413、code `body_too_large`。
- `GET /nope` 返回 404、code `not_found`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/httpapi`

Expected: FAIL，`New` 未定义。

- [ ] **Step 3: 实现处理器**

把下面的文件写成 `internal/httpapi/server.go`。限流是 60 秒固定窗口：某个键第一次计入时打开窗口，次数达到 `LoginLimit` 后拒绝，窗口过期后下一次请求重新计数。不读取转发头。

```go
package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"youchu/internal/auth"
	"youchu/internal/config"
)

type windowHit struct {
	start time.Time
	n     int
}

type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string]windowHit
}

func NewLimiter(limit int, now func() time.Time) *Limiter {
	return &Limiter{limit: limit, window: time.Minute, now: now, hits: map[string]windowHit{}}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	hit := l.hits[key]
	if hit.start.IsZero() || now.Sub(hit.start) >= l.window {
		hit = windowHit{start: now}
	}
	if hit.n >= l.limit {
		l.hits[key] = hit
		return false
	}
	hit.n++
	l.hits[key] = hit
	return true
}

type handler struct {
	db        *sql.DB
	cfg       config.Config
	dummyHash string
	now       func() time.Time
	limit     *Limiter
}

func New(db *sql.DB, cfg config.Config, dummyHash string, now func() time.Time) http.Handler {
	h := &handler{db: db, cfg: cfg, dummyHash: dummyHash, now: now, limit: NewLimiter(cfg.LoginLimit, now)}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/session", h.session)
	mux.HandleFunc("/api/v1/me", h.me)
	mux.HandleFunc("/", h.notFound)
	return mux
}

func (h *handler) session(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.login(w, r)
	case http.MethodDelete:
		h.logout(w, r)
	default:
		h.notFound(w, r)
	}
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	body, ok := readLimited(w, r)
	if !ok {
		return
	}
	if !h.limit.Allow(clientKey(r.RemoteAddr)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "登录过于频繁")
		return
	}
	var creds struct {
		Username *string `json:"username"`
		Password *string `json:"password"`
	}
	if err := json.Unmarshal(body, &creds); err != nil || creds.Username == nil || creds.Password == nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	username := strings.TrimSpace(*creds.Username)
	if username == "" || *creds.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
		return
	}
	var hash string
	err := h.db.QueryRowContext(r.Context(), `SELECT password_hash FROM users WHERE username = ?`, username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		auth.Verify(h.dummyHash, *creds.Password)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if !auth.Verify(hash, *creds.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	token, err := auth.Create(r.Context(), h.db, h.cfg.SessionTTL, h.now())
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "youchu_session", Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: h.cfg.CookieSecure,
		MaxAge: int(h.cfg.SessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]string{"username": username})
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	if !h.originOK(w, r) {
		return
	}
	if _, ok := readLimited(w, r); !ok {
		return
	}
	if c, err := r.Cookie("youchu_session"); err == nil {
		_ = auth.Delete(r.Context(), h.db, c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: "youchu_session", Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: h.cfg.CookieSecure, MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.notFound(w, r)
		return
	}
	c, err := r.Cookie("youchu_session")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "未登录")
		return
	}
	name, err := auth.Lookup(r.Context(), h.db, c.Value, h.now())
	if errors.Is(err, auth.ErrUnauthenticated) {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "未登录")
		return
	}
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": name})
}

func (h *handler) originOK(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Origin") != h.cfg.PublicOrigin {
		writeError(w, http.StatusForbidden, "origin_rejected", "来源不被接受")
		return false
	}
	return true
}

func (h *handler) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "未找到")
}

func readLimited(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	body, err := io.ReadAll(r.Body)
	if err == nil {
		return body, true
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "请求正文过大")
		return nil, false
	}
	writeError(w, http.StatusBadRequest, "invalid_body", "请求格式不正确")
	return nil, false
}

func clientKey(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/httpapi`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi
git commit -m "feat: 提供登录、退出和当前用户接口"
```

### Task 9: 启动顺序

**Files:**
- Create: `internal/app/app_test.go`
- Create: `internal/app/app.go`
- Create: `cmd/youchu/main.go`

- [ ] **Step 1: 写失败测试**

```go
func TestBuildRejectsBadTTLBeforeCreatingDatabase(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{
		"YOUCHU_PUBLIC_ORIGIN": "http://127.0.0.1:5173",
		"YOUCHU_DATA_DIR":      dir,
		"YOUCHU_SESSION_TTL":   "0s",
		"YOUCHU_USERNAME":      "ada",
		"YOUCHU_PASSWORD":      "correct-horse",
	}
	if _, _, err := Build(context.Background(), func(k string) string { return env[k] }); err == nil {
		t.Fatal("accepted 0s ttl")
	}
	if _, err := os.Stat(filepath.Join(dir, "youchu.db")); !os.IsNotExist(err) {
		t.Fatal("database was created")
	}
}
```

再写一个成功路径：合法环境变量调用 `Build` 后，`users` 有一行；用返回的 handler 可以登录。再次 `Build` 指向同一目录并传入不同密码，哈希不变。嵌入编号检查使用真实的 `migrations.FS`；另外用一个导出的 `Prepare(ctx, cfg, files)` 喂入缺号文件，断言临时目录里没有 `youchu.db`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/app`

Expected: FAIL，`Build` 未定义。

- [ ] **Step 3: 实现组装和 main**

`internal/app/app.go`：

```go
package app

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"path/filepath"
	"time"

	"youchu/internal/auth"
	"youchu/internal/config"
	"youchu/internal/httpapi"
	"youchu/internal/migrate"
	"youchu/internal/store"
	"youchu/migrations"
)

func Build(ctx context.Context, getenv func(string) string) (*http.Server, *sql.DB, error) {
	cfg, err := config.Load(getenv)
	if err != nil {
		return nil, nil, err
	}
	files, err := LoadMigrations(migrations.FS)
	if err != nil {
		return nil, nil, err
	}
	db, err := Prepare(ctx, cfg, files)
	if err != nil {
		return nil, nil, err
	}
	dummy, err := auth.NewDummyHash()
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(db, cfg, dummy, time.Now)}, db, nil
}

func Prepare(ctx context.Context, cfg config.Config, files []migrate.File) (*sql.DB, error) {
	if err := migrate.CheckEmbedded(files); err != nil {
		return nil, err
	}
	db, err := store.Open(filepath.Join(cfg.DataDir, "youchu.db"))
	if err != nil {
		return nil, err
	}
	if err := migrate.Apply(ctx, db, files, time.Now()); err != nil {
		db.Close()
		return nil, err
	}
	if err := auth.Bootstrap(ctx, db, cfg.Username, cfg.Password); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func LoadMigrations(fsys fs.FS) ([]migrate.File, error) {
	var files []migrate.File
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		version, err := migrate.ParseVersion(path)
		if err != nil {
			return err
		}
		body, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		files = append(files, migrate.File{Version: version, Name: path, SQL: string(body)})
		return nil
	})
	return files, err
}
```

`cmd/youchu/main.go` 调用 `app.Build(context.Background(), os.Getenv)`。配置或迁移失败时把错误写到 stderr 并 `os.Exit(1)`，不调用 `ListenAndServe`。成功后在 goroutine 里 `ListenAndServe`。收到 SIGINT 或 SIGTERM 时用 5 秒超时调用 `Shutdown`，然后 `db.Close()`。日志里不写密码、令牌、Cookie 或密码哈希。

缺号测试调用 `Prepare`，文件为 `[]migrate.File{{Version: 1}, {Version: 3}}`，数据目录里不应出现 `youchu.db`。成功路径对同一目录调用两次 `Build`，第二次使用不同密码，`password_hash` 保持不变。

- [ ] **Step 4: 运行全部 Go 测试**

Run: `go test ./...`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/app cmd/youchu
git commit -m "feat: 按固定顺序启动登录服务"
```

### Task 10: 登录页

**Files:**
- Create: `web/package.json`
- Create: `web/package-lock.json`
- Create: `web/tsconfig.json`
- Create: `web/vite.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/api.ts`
- Create: `web/src/styles.module.css`
- Create: `web/src/vite-env.d.ts`
- Create: `README.md`

- [ ] **Step 1: 安装固定版本**

在 `web/` 执行：

```bash
npm install react@19.3.0 react-dom@19.3.0 react-router@8.4.0 @tanstack/react-query@5.104.0
npm install -D vite@8.3.1 typescript@7.0.2 @vitejs/plugin-react@6.1.1 @types/react@19.3.0 @types/react-dom@19.3.0
```

`package.json` 的 scripts：`"dev": "vite"`，`"build": "tsc --noEmit && vite build"`。不要使用 `latest`。

- [ ] **Step 2: 写 Vite 与页面**

`vite.config.ts` 把开发服务器固定在 `127.0.0.1:5173`，并把 `/api` 代理到 `http://127.0.0.1:8080`。

`api.ts` 使用 `credentials: "same-origin"`。`GET /api/v1/me` 在 401 时返回 `null`。登录 `POST /api/v1/session` 发送 JSON。失败时抛出响应里的 `message`。退出使用 `DELETE /api/v1/session`。

`App.tsx` 使用 `BrowserRouter`、`Routes`、`Route`，来自 `react-router`。`QueryClientProvider` 包在路由外。

- `/login`：已登录则 `Navigate` 到 `/`。表单有用户名、密码、登录按钮。密码框 `type="password"`，`autoComplete` 分别为 `username` 和 `current-password`。标签文字是“用户名”和“密码”，按钮是“登录”。失败时显示接口 message。
- `/`：查询尚未完成时显示“正在确认登录状态”。结果为 `null` 时进入 `/login`。已登录时显示用户名和“退出”按钮。退出成功后查询结果改为 `null` 并进入 `/login`。

样式放在 CSS Module 中，只使用表单、按钮和一段文字。

- [ ] **Step 3: 写 README**

说明两个进程：

```bash
YOUCHU_PUBLIC_ORIGIN=http://127.0.0.1:5173 \
YOUCHU_USERNAME=ada \
YOUCHU_PASSWORD=correct-horse \
go run ./cmd/youchu
```

另开终端在 `web/` 运行 `npm run dev`，浏览器打开 `http://127.0.0.1:5173`。写明不要把 `localhost` 和 `127.0.0.1` 混用，账号创建后应从长期环境去掉明文密码，Vite 代理会使登录限流按代理地址合并。

- [ ] **Step 4: 构建网页**

Run: `npm run build`，工作目录是 `web/`。

Expected: 退出码 0。

- [ ] **Step 5: 用浏览器核对**

启动 Go 和 Vite。完成这些操作：登录、看到用户名、退出、刷新后回到登录页、用错误密码看到“用户名或密码错误”。然后停掉两个进程。

- [ ] **Step 6: Commit**

```bash
git add web README.md
git commit -m "feat: 增加登录页和登录后的占位页"
```

不要提交 `web/node_modules` 或 `data/`。

### Task 11: 规格验收

- [ ] **Step 1: 跑全部 Go 测试**

Run: `go test ./...`

Expected: PASS，覆盖规格第 11 节的 16 项 Go 测试。

- [ ] **Step 2: 对照规格检查范围**

仓库中没有物品、位置、分类、照片、MCP、OAuth、Docker 实现，也没有默认密码。`go.sum` 与 `web/package-lock.json` 已入库。

- [ ] **Step 3: Commit**

只有验收时修改了文档或测试，才提交那些改动。没有改动就不做空提交。
