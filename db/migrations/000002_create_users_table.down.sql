-- 1. Drop foreign key constraint and index from links table
DROP INDEX IF EXISTS idx_links_user_id;

ALTER TABLE IF EXISTS links DROP CONSTRAINT IF EXISTS fk_links_user_id;

-- 2. Drop unique indexes and users table
DROP INDEX IF EXISTS idx_users_username_lower;
DROP INDEX IF EXISTS idx_users_email_lower;
DROP TABLE IF EXISTS users;

