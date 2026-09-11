package migrations_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// migration pairs an up script with the down script that reverses it.
type migration struct {
	name string
	up   string
	down string
}

// discoverMigrations returns every migration in lexicographic order, which is
// also apply order given the numeric prefix convention (000001, 000002, ...).
//
// Discovering the files instead of naming them keeps every migration added
// later covered by these tests automatically.
func discoverMigrations(t *testing.T) []migration {
	t.Helper()

	upPaths, err := filepath.Glob(filepath.Join(".", "*.up.sql"))
	if err != nil {
		t.Fatalf("Failed to list migration files: %v", err)
	}
	if len(upPaths) == 0 {
		t.Fatal("No *.up.sql migration files found")
	}

	sort.Strings(upPaths)

	migrations := make([]migration, 0, len(upPaths))
	for _, upPath := range upPaths {
		name := strings.TrimSuffix(filepath.Base(upPath), ".up.sql")
		migrations = append(migrations, migration{
			name: name,
			up:   upPath,
			down: filepath.Join(".", name+".down.sql"),
		})
	}

	return migrations
}

func TestMigrationFilesExistAndAreNonEmpty(t *testing.T) {
	for _, m := range discoverMigrations(t) {
		t.Run(m.name, func(t *testing.T) {
			upBytes, err := os.ReadFile(m.up)
			if err != nil {
				t.Fatalf("Failed to read up migration file: %v", err)
			}
			if len(upBytes) == 0 {
				t.Fatal("Up migration file is empty")
			}

			downBytes, err := os.ReadFile(m.down)
			if err != nil {
				t.Fatalf("Failed to read down migration file: %v", err)
			}
			if len(downBytes) == 0 {
				t.Fatal("Down migration file is empty")
			}
		})
	}
}

func TestMigrationReversibilityStructure(t *testing.T) {
	createTableRegex := regexp.MustCompile(`(?i)CREATE\ TABLE\ (?:IF\ NOT\ EXISTS\ )?([a-zA-Z0-9_]+)`)
	dropTableRegex := regexp.MustCompile(`(?i)DROP\ TABLE\ (?:IF\ EXISTS\ )?([a-zA-Z0-9_]+)`)

	for _, m := range discoverMigrations(t) {
		t.Run(m.name, func(t *testing.T) {
			upBytes, err := os.ReadFile(m.up)
			if err != nil {
				t.Fatalf("Failed to read up migration file: %v", err)
			}
			downBytes, err := os.ReadFile(m.down)
			if err != nil {
				t.Fatalf("Failed to read down migration file: %v", err)
			}

			createdMatches := createTableRegex.FindAllStringSubmatch(string(upBytes), -1)
			droppedMatches := dropTableRegex.FindAllStringSubmatch(string(downBytes), -1)

			createdTables := make(map[string]bool)
			for _, match := range createdMatches {
				if len(match) > 1 {
					createdTables[strings.ToLower(match[1])] = true
				}
			}

			droppedTables := make(map[string]bool)
			for _, match := range droppedMatches {
				if len(match) > 1 {
					droppedTables[strings.ToLower(match[1])] = true
				}
			}

			// Not every migration creates a table: one that only adds a
			// trigger, function or constraint to an existing table, like
			// 000003_audit_immutability, is expected to find zero here. The
			// invariant that matters either way is the one below: whatever a
			// migration does create, its down script must drop.
			for table := range createdTables {
				if !droppedTables[table] {
					t.Errorf("Table '%s' created in up migration but not dropped in down migration", table)
				}
			}

			// Verify no binary columns (BYTEA or BLOB) exist in up migration
			upContentUpper := strings.ToUpper(string(upBytes))
			if strings.Contains(upContentUpper, "BYTEA") || strings.Contains(upContentUpper, "BLOB") {
				t.Errorf("Up migration contains binary columns (BYTEA/BLOB), violating zero-binary relational storage policy")
			}
		})
	}
}

// TestMigrationsRunCleanlyOnPostgres applies the whole migration set against a
// live database and rolls it back.
//
// Down scripts run in reverse order because later migrations reference tables
// created by earlier ones; dropping in apply order would fail on the foreign
// keys. The set is applied a second time so the database is left in the state
// the rest of the suite expects.
func TestMigrationsRunCleanlyOnPostgres(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping live postgres migration execution test")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("Postgres DB not reachable: %v; skipping live test", err)
	}

	migrations := discoverMigrations(t)

	upSQL := make([]string, len(migrations))
	downSQL := make([]string, len(migrations))
	for i, m := range migrations {
		up, err := os.ReadFile(m.up)
		if err != nil {
			t.Fatalf("Failed to read up migration %s: %v", m.name, err)
		}
		down, err := os.ReadFile(m.down)
		if err != nil {
			t.Fatalf("Failed to read down migration %s: %v", m.name, err)
		}
		upSQL[i] = string(up)
		downSQL[i] = string(down)
	}

	applyUp := func(stage string) {
		for i, m := range migrations {
			if _, err := db.Exec(upSQL[i]); err != nil {
				t.Fatalf("Failed to execute UP migration %s (%s): %v", m.name, stage, err)
			}
		}
	}

	applyDown := func(stage string, fatal bool) {
		for i, m := range slices.Backward(migrations) {
			if _, err := db.Exec(downSQL[i]); err != nil && fatal {
				t.Fatalf("Failed to execute DOWN migration %s (%s): %v", m.name, stage, err)
			}
		}
	}

	// 1. Run down to ensure clean slate. Failures are ignored: the tables may
	// legitimately not exist yet on a fresh database.
	applyDown("clean slate", false)

	// 2. Run up
	applyUp("initial apply")

	// 3. Run down (reversible check)
	applyDown("rollback", true)

	// 4. Run up again
	applyUp("re-apply")
}
