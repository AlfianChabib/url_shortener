-- 1. Create users table with UUIDv7 primary key
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Case-insensitive unique indexes for email and username
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email));
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_lower ON users (LOWER(username));

-- 2. Add foreign key constraint to links table
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_links_user_id'
    ) THEN
        ALTER TABLE links 
            ADD CONSTRAINT fk_links_user_id 
            FOREIGN KEY (user_id) 
            REFERENCES users(id) 
            ON DELETE SET NULL;
    END IF;
END $$;

-- Index for querying links by user_id
CREATE INDEX IF NOT EXISTS idx_links_user_id ON links(user_id) WHERE user_id IS NOT NULL;

