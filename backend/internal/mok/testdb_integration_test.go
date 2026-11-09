//go:build integration

package mok

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/Wei-Shaw/sub2api/internal/repository"
)

// openMigratedDB returns an empty database with every embedded migration applied,
// upstream's and the fork's, the same way the server does at startup.
//
// By default it starts PostgreSQL in Docker. Setting MOK_TEST_POSTGRES_DSN to a URL such as
// postgres://user:pass@127.0.0.1:5432/postgres?sslmode=disable uses that server instead:
// each test gets a scratch database, dropped afterwards. The user needs CREATEDB.
func openMigratedDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MOK_TEST_POSTGRES_DSN")
	if dsn != "" {
		dsn = createScratchDatabase(t, dsn)
	} else {
		dsn = startPostgresContainer(t)
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	return db
}

func startPostgresContainer(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	if exec.CommandContext(ctx, "docker", "info").Run() != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("docker is not available (CI=true)")
		}
		t.Skip("docker is not available; start Docker or set MOK_TEST_POSTGRES_DSN to run this test")
	}
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("sub2api_mok_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	return dsn
}

func createScratchDatabase(t *testing.T, serverDSN string) string {
	t.Helper()
	u, err := url.Parse(serverDSN)
	require.NoError(t, err, "MOK_TEST_POSTGRES_DSN must be a postgres:// URL")
	admin, err := sql.Open("postgres", serverDSN)
	require.NoError(t, err)
	name := fmt.Sprintf("mok_test_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	// Registered first, so it runs after the test's own connection is closed.
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
		_ = admin.Close()
	})
	u.Path = "/" + name
	return u.String()
}
