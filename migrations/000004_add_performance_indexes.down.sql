-- Rollback: Remove performance indexes

-- Tickets table indexes
DROP INDEX IF EXISTS idx_tickets_requester_id;
DROP INDEX IF EXISTS idx_tickets_assignee_id;
DROP INDEX IF EXISTS idx_tickets_status;
DROP INDEX IF EXISTS idx_tickets_priority;
DROP INDEX IF EXISTS idx_tickets_status_created_at;
DROP INDEX IF EXISTS idx_tickets_requester_created_at;
DROP INDEX IF EXISTS idx_tickets_created_at_desc;

-- Comments table indexes
DROP INDEX IF EXISTS idx_comments_ticket_id;
DROP INDEX IF EXISTS idx_comments_ticket_created_at;
DROP INDEX IF EXISTS idx_comments_author_id;

-- Users table indexes
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_organization_id;

-- User roles table indexes
DROP INDEX IF EXISTS idx_user_roles_user_id;
DROP INDEX IF EXISTS idx_user_roles_role_id;

-- Role permissions table indexes
DROP INDEX IF EXISTS idx_role_permissions_role_id;
DROP INDEX IF EXISTS idx_role_permissions_permission_id;
