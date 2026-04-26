package main

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/sqlchaos"
	"github.com/lib/pq"
)

// TestResponse_JSONSerialization verifies that our API response struct
// correctly serializes to JSON with the proper tags.
func TestResponse_JSONSerialization(t *testing.T) {
	resp := Response{
		Message: "URL successfully shortened and saved to database!",
		Short:   "http://short.ly/xyz123",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	jsonString := string(data)

	if !strings.Contains(jsonString, `"message":"URL successfully shortened`) {
		t.Errorf("Expected JSON to contain correct message, got: %s", jsonString)
	}
}

// TestSQLChaos_RealDatabaseIntegration tests the actual injection of latency
// into a live PostgreSQL database connection.
func TestSQLChaos_RealDatabaseIntegration(t *testing.T) {
	// 1. Force a guaranteed 200ms delay policy for database targets
	cfg := &config.PastaayConfig{
		Policies: []config.Policy{
			{Target: "database", Type: "sql", LatencyChance: 1.0, LatencyDuration: 200 * time.Millisecond},
		},
	}
	cfgManager := config.NewManager(cfg)

	// 2. Register the real Postgres driver wrapped in our chaos agent
	// We use a different name to avoid conflict with main.go during tests
	sqlchaos.Register("pastaay-postgres-test", &pq.Driver{}, cfgManager)

	// 3. Connect to the local Docker database
	db, err := sql.Open("pastaay-postgres-test", "postgres://pastaay:secret@localhost:5433/shortener?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// 4. Create a dummy table for the test
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS test_links (id SERIAL PRIMARY KEY, url TEXT)")
	if err != nil {
		t.Fatalf("Database not ready. Is docker-compose running? Error: %v", err)
	}

	// 5. Start the timer and execute the query
	start := time.Now()
	_, err = db.Exec("INSERT INTO test_links (url) VALUES ('http://test.ly')")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Insert query failed: %v", err)
	}

	// 6. Assert that the chaos wrapper actually delayed the query
	if elapsed < 200*time.Millisecond {
		t.Errorf("Expected query to take at least 200ms due to chaos injection, but it took only %v", elapsed)
	} else {
		t.Logf("Success! SQL Chaos delayed the query by %v", elapsed)
	}
}
