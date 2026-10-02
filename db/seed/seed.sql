-- Initial seed data for development
INSERT INTO links (id, short_code, original_url, is_active, created_at, expires_at)
VALUES (1000000000000001, 'google', 'https://www.google.com', true, NOW(), NULL)
ON CONFLICT (short_code) DO NOTHING;

