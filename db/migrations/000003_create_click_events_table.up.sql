-- Create click_events table for high-performance time-series analytics
CREATE TABLE IF NOT EXISTS click_events (
    id BIGSERIAL PRIMARY KEY,
    short_code VARCHAR(32) NOT NULL,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip_hash VARCHAR(64) NOT NULL,
    country_code VARCHAR(10) NOT NULL,
    city VARCHAR(100) NOT NULL,
    device_type VARCHAR(50) NOT NULL,
    browser VARCHAR(50) NOT NULL,
    os VARCHAR(50) NOT NULL,
    referer TEXT NOT NULL
);

-- Optimize queries for analytics aggregation by short_code and timestamp
CREATE INDEX IF NOT EXISTS idx_click_events_short_code_clicked_at ON click_events(short_code, clicked_at);
CREATE INDEX IF NOT EXISTS idx_click_events_short_code ON click_events(short_code);
