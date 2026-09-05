package migrations_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func TestMigrationFilesExistAndAreNonEmpty(t *testing.T) {
	upPath := filepath.Join(".", "000001_init_schema.up.sql")
	downPath := filepath.Join(".", "000001_init_schema.down.sql")

	upBytes, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatalf("Failed to read up migration file: %v", err)
	}
	if len(upBytes) == 0 {
		t.Fatal("Up migration file is empty")
	}

	downBytes, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatalf("Failed to read down migration file: %v", err)
	}
	if len(downBytes) == 0 {
		t.Fatal("Down migration file is empty")
	}
}

func TestMigrationReversibilityStructure(t *testing.T) {
	upPath := filepath.Join(".", "000001_init_schema.up.sql")
	downPath := filepath.Join(".", "000001_init_schema.down.sql")

	upBytes, _ := os.ReadFile(upPath)
	downBytes, _ := os.ReadFile(downPath)

	createTableRegex := regexp.MustCompile(`(?i)CREATE\ TABLE\ (?:IF\ NOT\ EXISTS\ )?([a-zA-Z0-9_]+)`)
	dropTableRegex := regexp.MustCompile(`(?i)DROP\ TABLE\ (?:IF\ EXISTS\ )?([a-zA-Z0-9_]+)`)

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

	if len(createdTables) == 0 {
		t.Fatal("No tables found in up migration")
	}

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
}

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

	upPath := filepath.Join(".", "000001_init_schema.up.sql")
	downPath := filepath.Join(".", "000001_init_schema.down.sql")

	upSQL, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatalf("Failed to read up migration: %v", err)
	}

	downSQL, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatalf("Failed to read down migration: %v", err)
	}

	// 1. Run down to ensure clean slate
	_, _ = db.Exec(string(downSQL))

	// 2. Run up
	if _, err := db.Exec(string(upSQL)); err != nil {
		t.Fatalf("Failed to execute UP migration: %v", err)
	}

	// 3. Run down (reversible check)
	if _, err := db.Exec(string(downSQL)); err != nil {
		t.Fatalf("Failed to execute DOWN migration: %v", err)
	}

	// 4. Run up again
	if _, err := db.Exec(string(upSQL)); err != nil {
		t.Fatalf("Failed to re-execute UP migration: %v", err)
	}
}
