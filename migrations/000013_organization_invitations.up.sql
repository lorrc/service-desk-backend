CREATE TABLE organization_invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token_hash BYTEA NOT NULL UNIQUE,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT organization_invitations_email_normalized
        CHECK (email <> '' AND email = lower(btrim(email))),
    CONSTRAINT organization_invitations_token_hash_length
        CHECK (octet_length(token_hash) = 32),
    CONSTRAINT organization_invitations_expiration
        CHECK (expires_at > created_at),
    CONSTRAINT organization_invitations_terminal_status
        CHECK (accepted_at IS NULL OR revoked_at IS NULL),
    CONSTRAINT organization_invitations_acceptance_time
        CHECK (accepted_at IS NULL OR accepted_at >= created_at),
    CONSTRAINT organization_invitations_revocation_time
        CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX idx_organization_invitations_organization_created
    ON organization_invitations (organization_id, created_at DESC, id DESC);
