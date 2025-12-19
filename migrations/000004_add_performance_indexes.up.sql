-- Migration: Add performance indexes for common query patterns
-- This migration adds indexes to improve query performance for tickets and comments

-- Tickets table indexes
-- Index for filtering by requester (customers viewing their tickets)
CREATE INDEX IF NOT EXISTS idx_tickets_requester_id ON tickets(requester_id);

-- Index for filtering by assignee (agents viewing assigned tickets)
CREATE INDEX IF NOT EXISTS idx_tickets_assignee_id ON tickets(assignee_id);

-- Index for filtering by status (common filter in list views)
CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);

-- Index for filtering by priority (common filter in list views)
CREATE INDEX IF NOT EXISTS idx_tickets_priority ON tickets(priority);

-- Composite index for common query pattern: status + created_at ordering
CREATE INDEX IF NOT EXISTS idx_tickets_status_created_at ON tickets(status, created_at DESC);

-- Composite index for requester queries with date ordering
CREATE INDEX IF NOT EXISTS idx_tickets_requester_created_at ON tickets(requester_id, created_at DESC);

-- Index for sorting by created_at (default sort order)
CREATE INDEX IF NOT EXISTS idx_tickets_created_at_desc ON tickets(created_at DESC);

-- Comments table indexes
-- Index for fetching comments by ticket (most common query)
CREATE INDEX IF NOT EXISTS idx_comments_ticket_id ON comments(ticket_id);

-- Composite index for comments ordered by creation time
CREATE INDEX IF NOT EXISTS idx_comments_ticket_created_at ON comments(ticket_id, created_at ASC);

-- Index for finding comments by author
CREATE INDEX IF NOT EXISTS idx_comments_author_id ON comments(author_id);

-- Users table indexes
-- Index for email lookups (login)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Index for organization-based queries
CREATE INDEX IF NOT EXISTS idx_users_organization_id ON users(organization_id);

-- User roles table indexes (for RBAC queries)
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);

-- Role permissions table indexes
CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);
