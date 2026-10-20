package migrations_test

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func testInvitationMigration(t *testing.T, connectionString string) {
	t.Run("version 13 preserves existing accounts and access rights", func(t *testing.T) {
		db := newMigrationDatabase(t, connectionString)
		db.to(12)
		userID, ticketID := db.fixture()
		db.exec("INSERT INTO user_roles SELECT $1, id FROM roles WHERE name = 'admin'", userID)
		identities, indexes := db.identitySnapshot(), db.indexes()
		db.to(13)
		require.Equal(t, identities, db.identitySnapshot())
		hash := sha256.Sum256([]byte("migration-test-invitation-code"))
		var invitationID uuid.UUID
		require.NoError(t, db.pool.QueryRow(t.Context(), `INSERT INTO organization_invitations
			(organization_id, email, token_hash, created_by, expires_at)
			VALUES ('00000000-0000-0000-0000-000000000001', 'invited@example.test', $1, $2, NOW() + interval '1 day')
			RETURNING id`, hash[:], userID).Scan(&invitationID))
		require.NotEqual(t, uuid.Nil, invitationID)
		require.Equal(t, int64(1), db.count(`SELECT count(*) FROM organization_invitations
			WHERE created_by = $1 AND token_hash = $2 AND accepted_at IS NULL AND revoked_at IS NULL AND created_at < expires_at`, userID, hash[:]))
		require.Equal(t, int64(1), db.count(`SELECT count(*) FROM pg_indexes WHERE schemaname = 'public'
			AND indexname = 'idx_organization_invitations_organization_created'
			AND indexdef LIKE '%(organization_id, created_at DESC, id DESC)%'`))
		require.Equal(t, int64(0), db.count(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'organization_invitations' AND column_name IN ('token', 'code', 'role')`))
		db.to(12)
		require.Equal(t, identities, db.identitySnapshot())
		require.Equal(t, indexes, db.indexes())
		db.assertFixture(userID, ticketID)
		require.Equal(t, int64(0), db.count(`SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'organization_invitations'`))
		db.to(13)
		require.Equal(t, identities, db.identitySnapshot())
		require.Equal(t, int64(0), db.count("SELECT count(*) FROM organization_invitations"))
	})

	t.Run("version 13 enforces invitation token and lifecycle invariants", func(t *testing.T) {
		db := newMigrationDatabase(t, connectionString)
		db.to(13)
		userID, _ := db.fixture()
		orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		created := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
		expires := created.Add(24 * time.Hour)
		accepted := created.Add(time.Hour)
		tooEarly := created.Add(-time.Second)
		hash := sha256.Sum256([]byte("valid-invitation-token"))
		const insert = `INSERT INTO organization_invitations
			(organization_id, email, token_hash, created_by, created_at, expires_at, accepted_at, revoked_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
		cases := []struct {
			name, email, code string
			hash              []byte
			organization      uuid.UUID
			creator           uuid.UUID
			expires           time.Time
			accepted, revoked *time.Time
		}{
			{name: "uppercase email", email: "Invited@example.test", code: "23514"},
			{name: "surrounding spaces", email: " invited@example.test ", code: "23514"},
			{name: "blank email", email: "", code: "23514"},
			{name: "short hash", email: "invited@example.test", hash: make([]byte, 31), code: "23514"},
			{name: "long hash", email: "invited@example.test", hash: make([]byte, 33), code: "23514"},
			{name: "expires at creation", email: "invited@example.test", expires: created, code: "23514"},
			{name: "expires before creation", email: "invited@example.test", expires: tooEarly, code: "23514"},
			{name: "accepted and revoked", email: "invited@example.test", accepted: &accepted, revoked: &accepted, code: "23514"},
			{name: "accepted before creation", email: "invited@example.test", accepted: &tooEarly, code: "23514"},
			{name: "revoked before creation", email: "invited@example.test", revoked: &tooEarly, code: "23514"},
			{name: "missing organization", email: "invited@example.test", organization: uuid.New(), code: "23503"},
			{name: "missing creator", email: "invited@example.test", creator: uuid.New(), code: "23503"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.hash == nil {
					tc.hash = hash[:]
				}
				if tc.expires.IsZero() {
					tc.expires = expires
				}
				if tc.organization == uuid.Nil {
					tc.organization = orgID
				}
				if tc.creator == uuid.Nil {
					tc.creator = userID
				}
				_, err := db.pool.Exec(t.Context(), insert, tc.organization, tc.email, tc.hash, tc.creator, created, tc.expires, tc.accepted, tc.revoked)
				var postgresError *pgconn.PgError
				require.ErrorAs(t, err, &postgresError)
				require.Equal(t, tc.code, postgresError.Code)
			})
		}
		_, err := db.pool.Exec(t.Context(), insert, orgID, "invited@example.test", nil, userID, created, expires, nil, nil)
		var postgresError *pgconn.PgError
		require.ErrorAs(t, err, &postgresError)
		require.Equal(t, "23502", postgresError.Code, "a token hash is required")
		db.exec(insert, orgID, "invited@example.test", hash[:], userID, created, expires, nil, nil)
		_, err = db.pool.Exec(t.Context(), insert, orgID, "another@example.test", hash[:], userID, created, expires, nil, nil)
		require.ErrorAs(t, err, &postgresError)
		require.Equal(t, "23505", postgresError.Code, "hashes must be globally unique")
		db.exec("UPDATE organization_invitations SET accepted_at = $1 WHERE token_hash = $2", accepted, hash[:])
		_, err = db.pool.Exec(t.Context(), "UPDATE organization_invitations SET revoked_at = $1 WHERE token_hash = $2", accepted, hash[:])
		require.ErrorAs(t, err, &postgresError)
		require.Equal(t, "23514", postgresError.Code)
		secondHash := sha256.Sum256([]byte("replacement-invitation-token"))
		db.exec(insert, orgID, "invited@example.test", secondHash[:], userID, created, expires, nil, &accepted)
		require.Equal(t, int64(2), db.count("SELECT count(*) FROM organization_invitations"), "different codes can retain the same email's invitation history")
	})
}

func (db *migrationDatabase) identitySnapshot() string {
	return db.text(`SELECT jsonb_build_object(
		'organizations', (SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM organizations o),
		'users', (SELECT jsonb_agg(to_jsonb(u) ORDER BY id) FROM users u),
		'roles', (SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM roles r),
		'permissions', (SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM permissions p),
		'user_roles', (SELECT jsonb_agg(to_jsonb(ur) ORDER BY user_id, role_id) FROM user_roles ur),
		'role_permissions', (SELECT jsonb_agg(to_jsonb(rp) ORDER BY role_id, permission_id) FROM role_permissions rp)
	)::text`)
}
