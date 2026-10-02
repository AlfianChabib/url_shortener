package web

import "time"

type CountryClick struct {
	Code   string `json:"code"`
	Clicks int64  `json:"clicks"`
}

type DevicesStats struct {
	Mobile  float64 `json:"mobile"`
	Desktop float64 `json:"desktop"`
	Bot     float64 `json:"bot"`
}

type TimeSeriesData struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int64     `json:"count"`
}

// AnalyticsResponse defines the payload returned by the analytics endpoint.
type AnalyticsResponse struct {
	ShortCode    string           `json:"short_code"`
	TotalClicks  int64            `json:"total_clicks"`
	UniqueClicks int64            `json:"unique_clicks"`
	TopCountries []CountryClick   `json:"top_countries"`
	Devices      DevicesStats     `json:"devices"`
	TimeSeries   []TimeSeriesData `json:"timeseries"`
}

