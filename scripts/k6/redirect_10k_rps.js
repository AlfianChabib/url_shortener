import http from 'k6/http';
import { check, sleep } from 'k6';

// k6 Load Testing Script for High-Performance URL Shortener - Phase 3 Benchmarking
// Target: 10,000 RPS on Read / Redirect endpoint (GET /{short_code})
// SLO: P50 < 3ms, P99 < 15ms, Error Rate < 0.1%

export const options = {
  scenarios: {
    redirect_throughput_test: {
      executor: 'ramping-arrival-rate',
      startRate: 1000,
      timeUnit: '1s',
      preAllocatedVUs: 500,
      maxVUs: 3000,
      stages: [
        { duration: '30s', target: 5000 },  // Warm-up to 5,000 RPS
        { duration: '1m', target: 10000 },  // Ramp to 10,000 RPS target
        { duration: '3m', target: 10000 },  // Sustain 10,000 RPS load
        { duration: '30s', target: 0 },     // Graceful ramp-down
      ],
    },
  },
  thresholds: {
    // PRD Non-Functional Requirements (SLIs/SLOs)
    'http_req_duration{status:307}': [
      'p(50)<3',    // P50 < 3ms
      'p(99)<15',   // P99 < 15ms
    ],
    'http_req_duration': [
      'p(50)<5',
      'p(99)<20',
    ],
    'http_req_failed': ['rate<0.001'], // < 0.1% errors allowed
    'checks': ['rate>0.999'],
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:3000';
const SHORT_CODE = __ENV.SHORT_CODE || 'promo-sale-2026';

export function setup() {
  // Pre-seed test link before starting benchmark
  const payload = JSON.stringify({
    original_url: 'https://example.com/target-landing-page',
    custom_alias: SHORT_CODE,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  // Create link if not already existing
  http.post(`${BASE_URL}/api/v1/links`, payload, params);

  return { shortCode: SHORT_CODE };
}

export default function (data) {
  // Disable automatic redirect following so we accurately measure redirection latency
  const params = {
    redirects: 0,
    headers: {
      'User-Agent': 'k6-load-testing-agent/1.0',
      'X-Forwarded-For': `203.0.113.${Math.floor(Math.random() * 250) + 1}`,
      'CF-IPCountry': 'ID',
    },
  };

  const res = http.get(`${BASE_URL}/${data.shortCode}`, params);

  check(res, {
    'status is 307 Temporary Redirect': (r) => r.status === 307,
    'has Location header': (r) => r.headers['Location'] !== undefined,
  });
}
