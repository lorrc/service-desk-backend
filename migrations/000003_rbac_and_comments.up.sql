-- RBAC Tables
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id INTEGER REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INTEGER REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- Comments Table
CREATE TABLE IF NOT EXISTS comments (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id),
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed basic roles and permissions
INSERT INTO permissions (name) VALUES 
    ('tickets:create'), 
    ('tickets:read'), 
    ('tickets:read:all'), 
    ('tickets:update:status'), 
    ('tickets:assign'), 
    ('comments:create'), 
    ('comments:read')
ON CONFLICT DO NOTHING;

INSERT INTO roles (name) VALUES ('admin'), ('agent'), ('customer') ON CONFLICT DO NOTHING;

-- Assign all permissions to admin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- Agent gets most permissions except admin-only ones
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p 
WHERE r.name = 'agent' AND p.code IN (
    'tickets:create', 'tickets:read', 'tickets:read:all', 
    'tickets:update:status', 'tickets:assign', 'tickets:list:all',
    'comments:create', 'comments:read'
)
ON CONFLICT DO NOTHING;

-- Customer gets basic permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p 
WHERE r.name = 'customer' AND p.code IN (
    'tickets:create', 'tickets:read', 'comments:create', 'comments:read'
)
ON CONFLICT DO NOTHING;
