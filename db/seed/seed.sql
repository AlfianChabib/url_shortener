-- 1. Initial seed user for development (password: Password123!)
-- Bcrypt hash generated with cost 10 for 'Password123!'
INSERT INTO users (id, email, username, password_hash, is_active, created_at, updated_at)
VALUES (
    '018f3a2b-8a71-7000-8000-000000000001',
    'developer@example.com',
    'developer',
    '$2a$10$7v59nFqf1J6kQ6YpZJqIe.K9mK/mZ2LqO7C9t1H9vK7zK2mY8C4G2',
    true,
    NOW(),
    NOW()
)
ON CONFLICT (email) DO NOTHING;

-- 2. Initial seed short link associated with development user
INSERT INTO links (id, short_code, original_url, user_id, is_active, created_at, expires_at)
VALUES (
    '018f3a2b-8a71-7000-8000-000000000002',
    'google',
    'https://www.google.com',
    '018f3a2b-8a71-7000-8000-000000000001',
    true,
    NOW(),
    NULL
)
ON CONFLICT (short_code) DO NOTHING;
