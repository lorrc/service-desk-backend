package migrations_test

import (
	"context"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Each case uses a new database in one disposable container. Cleanup belongs to
// testing.T rather than a TestMain defer that os.Exit would skip.
func TestMigrationSafety(t *testing.T) {
	container, err := postgres.RunContainer(t.Context(),
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("migration-tests"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		require.NoError(t, container.Terminate(ctx))
	})
	connectionString, err := container.ConnectionString(t.Context(), "sslmode=disable")
	require.NoError(t, err)

	t.Run("version 2 removes only its indexes", func(t *testing.T) {
		db := newMigrationDatabase(t, connectionString)
		db.to(1)
		userID, ticketID := db.fixture()
		indexes := db.indexes()
		db.to(2)
		require.Len(t, db.indexes(), len(indexes)+4)
		db.to(1)
		require.Equal(t, indexes, db.indexes())
		db.assertFixture(userID, ticketID)
		db.to(2)
		require.Len(t, db.indexes(), len(indexes)+4)
	})

	t.Run("version 3 removes RBAC and comments in dependency order", func(t *testing.T) {
		db := newMigrationDatabase(t, connectionString)
		db.to(2)
		userID, ticketID := db.fixture()
		db.to(3)
		db.exec("INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'before downgrade')", ticketID, userID)
		db.exec("INSERT INTO user_roles SELECT $1, id FROM roles WHERE name = 'customer'", userID)
		db.to(2)
		db.assertFixture(userID, ticketID)
		require.Equal(t, int64(0), db.count(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name IN ('comments', 'user_roles', 'role_permissions', 'permissions', 'roles')`))
		db.to(3)
		require.Equal(t, int64(3), db.count("SELECT count(*) FROM roles"))
		db.exec("INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'after upgrade')", ticketID, userID)
	})

	t.Run("version 4 preserves earlier indexes and data", func(t *testing.T) {
		db := newMigrationDatabase(t, connectionString)
		db.to(3)
		userID, ticketID := db.fixture()
		db.exec("INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'keep me')", ticketID, userID)
		indexes := db.indexes()
		comments := db.comments()
		db.to(4)
		require.Greater(t, len(db.indexes()), len(indexes))
		db.to(3)
		require.Equal(t, indexes, db.indexes())
		require.Equal(t, comments, db.comments())
		db.assertFixture(userID, ticketID)
	})

	t.Run("version 3 failed downgrade does not partially remove tables", func(t *testing.T) {
		db := newMigrationDatabase(t, connectionString)
		db.to(3)
		userID, ticketID := db.fixture()
		db.exec("INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'keep on failure')", ticketID, userID)
		db.exec("INSERT INTO user_roles SELECT $1, id FROM roles WHERE name = 'customer'", userID)
		db.exec("CREATE VIEW role_audit AS SELECT name FROM roles")
		indexes, comments := db.indexes(), db.comments()
		err := db.migration.Migrate(2)
		require.ErrorContains(t, err, "depend")
		require.Equal(t, comments, db.comments())
		require.Equal(t, indexes, db.indexes())
		require.Equal(t, int64(1), db.count("SELECT count(*) FROM user_roles WHERE user_id = $1", userID))
		require.Equal(t, int64(3), db.count("SELECT count(*) FROM roles"))
		db.assertFixture(userID, ticketID)
	})

	t.Run("version 6 preserves existing permissions and customer grants", func(t *testing.T) {
		db := newMigrationDatabase(t, connectionString)
		db.to(5)
		existingCustomer, _ := db.fixture()
		unassignedUser, _ := db.fixture()
		db.exec("INSERT INTO user_roles SELECT $1, id FROM roles WHERE name = 'customer'", existingCustomer)
		permissions := db.text(`SELECT jsonb_agg(jsonb_build_array(r.name, p.code) ORDER BY r.name, p.code)::text
			FROM role_permissions rp JOIN roles r ON r.id = rp.role_id JOIN permissions p ON p.id = rp.permission_id`)
		db.to(6)
		db.to(5)
		require.Equal(t, permissions, db.text(`SELECT jsonb_agg(jsonb_build_array(r.name, p.code) ORDER BY r.name, p.code)::text
			FROM role_permissions rp JOIN roles r ON r.id = rp.role_id JOIN permissions p ON p.id = rp.permission_id`))
		for _, userID := range []uuid.UUID{existingCustomer, unassignedUser} {
			require.Equal(t, int64(1), db.count(`SELECT count(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id
				WHERE ur.user_id = $1 AND r.name = 'customer'`, userID))
		}
		db.to(6)
		require.Equal(t, int64(2), db.count("SELECT count(*) FROM user_roles"))
	})

	for _, populated := range []bool{false, true} {
		name := "empty"
		if populated {
			name = "populated"
		}
		t.Run("version 11 preserves canonical "+name+" comments", func(t *testing.T) {
			db := newMigrationDatabase(t, connectionString)
			db.to(10)
			userID, ticketID := db.fixture()
			if populated {
				db.exec("INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'first'), ($1, $2, 'second')", ticketID, userID)
			}
			comments, indexes := db.comments(), db.indexes()
			db.to(11)
			db.to(10)
			require.Equal(t, comments, db.comments())
			require.Equal(t, indexes, db.indexes())
			require.Equal(t, "bigint", db.commentIDType())
			var nextID int64
			require.NoError(t, db.pool.QueryRow(t.Context(),
				"INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'after downgrade') RETURNING id", ticketID, userID).Scan(&nextID))
			expectedID := int64(1)
			if populated {
				expectedID = 3
			}
			require.Equal(t, expectedID, nextID)
			db.to(11)
			require.Equal(t, expectedID, db.count("SELECT count(*) FROM comments"))
		})

		t.Run("version 11 upgrades "+name+" legacy UUID comments safely", func(t *testing.T) {
			db := newMigrationDatabase(t, connectionString)
			db.to(10)
			userID, ticketID := db.fixture()
			db.exec("ALTER TABLE comments DROP COLUMN id; ALTER TABLE comments ADD COLUMN id UUID PRIMARY KEY DEFAULT uuid_generate_v4()")
			if populated {
				db.exec("INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'first'), ($1, $2, 'second')", ticketID, userID)
			}
			contents := db.text("SELECT COALESCE(jsonb_agg(to_jsonb(c) - 'id' ORDER BY body), '[]')::text FROM comments c")
			db.to(11)
			require.Equal(t, "bigint", db.commentIDType())
			require.Equal(t, contents, db.text("SELECT COALESCE(jsonb_agg(to_jsonb(c) - 'id' ORDER BY body), '[]')::text FROM comments c"))
			var nextID int64
			require.NoError(t, db.pool.QueryRow(t.Context(),
				"INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'after conversion') RETURNING id", ticketID, userID).Scan(&nextID))
			expectedID := int64(1)
			if populated {
				expectedID = 3
			}
			require.Equal(t, expectedID, nextID)
			converted := db.comments()
			err := db.migration.Migrate(10)
			require.ErrorContains(t, err, "irreversible")
			require.Equal(t, "bigint", db.commentIDType())
			require.Equal(t, converted, db.comments(), "rejected rollback must not change IDs or comment content")
			db.exec("INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'after rejected rollback')", ticketID, userID)
		})

		t.Run("complete "+name+" database rebuild", func(t *testing.T) {
			db := newMigrationDatabase(t, connectionString)
			require.NoError(t, db.migration.Up())
			if populated {
				userID, ticketID := db.fixture()
				db.exec("INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'before rebuild')", ticketID, userID)
				db.exec("INSERT INTO ticket_events (ticket_id, type, payload, actor_id) VALUES ($1, 'created', '{}', $2)", ticketID, userID)
			}
			require.NoError(t, db.migration.Down())
			require.Equal(t, int64(0), db.count(`SELECT count(*) FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name <> 'schema_migrations'`))
			require.NoError(t, db.migration.Up())
			userID, ticketID := db.fixture()
			db.exec("INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'after rebuild')", ticketID, userID)
			require.Equal(t, int64(1), db.count("SELECT id FROM comments"))
			require.Equal(t, int64(1), db.count("SELECT session_version FROM users WHERE id = $1", userID))
			require.Equal(t, int64(1), db.count(`SELECT count(*) FROM role_permissions rp
				JOIN roles r ON r.id = rp.role_id JOIN permissions p ON p.id = rp.permission_id
				WHERE r.name = 'admin' AND p.code = 'admin:access'`))
		})
	}
}

type migrationDatabase struct {
	t         *testing.T
	pool      *pgxpool.Pool
	migration *migrate.Migrate
}

func newMigrationDatabase(t *testing.T, connectionString string) *migrationDatabase {
	t.Helper()
	admin, err := pgx.Connect(t.Context(), connectionString)
	require.NoError(t, err)
	name := "migration_" + uuid.NewString()
	_, err = admin.Exec(t.Context(), "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	require.NoError(t, admin.Close(t.Context()))
	databaseURL, err := url.Parse(connectionString)
	require.NoError(t, err)
	databaseURL.Path = "/" + name
	migrationsPath, err := filepath.Abs(".")
	require.NoError(t, err)
	migration, err := migrate.New("file://"+migrationsPath, databaseURL.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		sourceErr, databaseErr := migration.Close()
		require.NoError(t, sourceErr)
		require.NoError(t, databaseErr)
	})
	pool, err := pgxpool.New(t.Context(), databaseURL.String())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return &migrationDatabase{t: t, pool: pool, migration: migration}
}

func (db *migrationDatabase) to(version uint) {
	db.t.Helper()
	require.NoError(db.t, db.migration.Migrate(version))
}

func (db *migrationDatabase) exec(sql string, args ...interface{}) {
	db.t.Helper()
	_, err := db.pool.Exec(db.t.Context(), sql, args...)
	require.NoError(db.t, err)
}

func (db *migrationDatabase) count(sql string, args ...interface{}) int64 {
	db.t.Helper()
	var value int64
	require.NoError(db.t, db.pool.QueryRow(db.t.Context(), sql, args...).Scan(&value))
	return value
}

func (db *migrationDatabase) text(sql string) string {
	db.t.Helper()
	var value string
	require.NoError(db.t, db.pool.QueryRow(db.t.Context(), sql).Scan(&value))
	return value
}

func (db *migrationDatabase) fixture() (uuid.UUID, int64) {
	db.t.Helper()
	userID := uuid.New()
	db.exec(`INSERT INTO users (id, organization_id, full_name, email, hashed_password)
		VALUES ($1, '00000000-0000-0000-0000-000000000001', 'Migration test', $2, 'not-a-login-password')`,
		userID, userID.String()+"@example.test")
	var ticketID int64
	require.NoError(db.t, db.pool.QueryRow(db.t.Context(),
		"INSERT INTO tickets (title, requester_id) VALUES ('Migration test', $1) RETURNING id", userID).Scan(&ticketID))
	return userID, ticketID
}

func (db *migrationDatabase) assertFixture(userID uuid.UUID, ticketID int64) {
	db.t.Helper()
	require.Equal(db.t, int64(1), db.count("SELECT count(*) FROM users WHERE id = $1", userID))
	require.Equal(db.t, int64(1), db.count("SELECT count(*) FROM tickets WHERE id = $1 AND requester_id = $2", ticketID, userID))
}

func (db *migrationDatabase) indexes() []string {
	db.t.Helper()
	rows, err := db.pool.Query(db.t.Context(), `SELECT indexdef FROM pg_indexes
		WHERE schemaname = 'public' AND tablename <> 'schema_migrations' ORDER BY indexname`)
	require.NoError(db.t, err)
	indexes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(db.t, err)
	return indexes
}

func (db *migrationDatabase) comments() string {
	return db.text("SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY id), '[]')::text FROM comments c")
}

func (db *migrationDatabase) commentIDType() string {
	return db.text(`SELECT data_type FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'comments' AND column_name = 'id'`)
}
