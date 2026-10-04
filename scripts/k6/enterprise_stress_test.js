import http from 'k6/http';
import { check, group } from 'k6';

// k6 Full System Enterprise Stress Test (Phase 3 Enterprise Hardening)
// Tests concurrent redirect traffic, link creation, and authentication under sustained load.

export const options = {
  scenarios: {
    // 1. High-Throughput Redirections (90% traffic)
    redirect_traffic: {
      executor: 'constant-arrival-rate',
      rate: 10000,
      timeUnit: '1s',
      duration: '1m',
      preAllocatedVUs: 1000,
      maxVUs: 3000,
      exec: 'testRedirect',
    },
    // 2. High-Throughput URL Shortening (10% traffic)
    creation_traffic: {
      executor: 'constant-arrival-rate',
      rate: 1000,
      timeUnit: '1s',
      duration: '1m',
      preAllocatedVUs: 200,
      maxVUs: 1000,
      exec: 'testCreation',
    },
  },
  thresholds: {
    'http_req_duration{scenario:redirect_traffic}': ['p(50)<3', 'p(99)<15'],
    'http_req_duration{scenario:creation_traffic}': ['p(99)<80'],
    'http_req_failed': ['rate<0.01'],
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:3000';

export function setup() {
  // Pre-seed test links
  const seeds = ['promo-1', 'promo-2', 'promo-3', 'campaign-spring'];
  for (const s of seeds) {
    http.post(
      `${BASE_URL}/api/v1/links`,
      JSON.stringify({
        original_url: `https://example.com/target/${s}`,
        custom_alias: s,
      }),
      { headers: { 'Content-Type': 'application/json' } }
    );
  }
  return { seeds };
}

export function testRedirect(data) {
  const shortCode = data.seeds[Math.floor(Math.random() * data.seeds.length)];
  const randomIP = `103.247.${Math.floor(Math.random() * 254) + 1}.${Math.floor(Math.random() * 254) + 1}`;

  const res = http.get(`${BASE_URL}/${shortCode}`, {
    redirects: 0,
    headers: {
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',
      'X-Forwarded-For': randomIP,
      'CF-IPCountry': 'ID',
    },
  });

  check(res, {
    'redirect is 307': (r) => r.status === 307,
    'has Cache-Control': (r) => r.headers['Cache-Control'] !== undefined,
  });
}

export function testCreation() {
  const rand = Math.floor(Math.random() * 10000000);
  const randomIP = `198.51.${Math.floor(Math.random() * 254) + 1}.${Math.floor(Math.random() * 254) + 1}`;

  const res = http.post(
    `${BASE_URL}/api/v1/links`,
    JSON.stringify({
      original_url: `https://example.com/items/${rand}`,
    }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-Forwarded-For': randomIP,
      },
    }
  );

  check(res, {
    'link created status 201': (r) => r.status === 201,
  });
}
