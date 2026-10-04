import http from "k6/http";
import { check } from "k6";

// k6 Load Testing Script for High-Performance URL Shortener - Phase 3 Benchmarking
// Target: 1,000 RPS on Write / Creation endpoint (POST /api/v1/links)
// SLO: P99 < 80ms, Error Rate < 0.5%

export const options = {
  scenarios: {
    creation_throughput_test: {
      executor: "ramping-arrival-rate",
      startRate: 100,
      timeUnit: "1s",
      preAllocatedVUs: 200,
      maxVUs: 1000,
      stages: [
        { duration: "30s", target: 500 }, // Warm-up to 500 RPS
        { duration: "1m", target: 1000 }, // Ramp to 1,000 RPS target
        { duration: "2m", target: 1000 }, // Sustain 1,000 RPS
        { duration: "30s", target: 0 }, // Ramp down
      ],
    },
  },
  thresholds: {
    // PRD Non-Functional Requirements: P99 < 80ms for link creation
    "http_req_duration{status:201}": ["p(99)<80"],
    http_req_duration: ["p(99)<100"],
    http_req_failed: ["rate<0.005"],
    checks: ["rate>0.995"],
  },
};

const BASE_URL = __ENV.TARGET_URL || "http://localhost:3000";

export default function () {
  const randomSuffix = Math.floor(Math.random() * 100000000);
  const payload = JSON.stringify({
    original_url: `https://yannn.fun/${randomSuffix}`,
    expires_in_hours: 168,
  });

  const clientIP = `198.51.${Math.floor(Math.random() * 250) + 1}.${Math.floor(Math.random() * 250) + 1}`;
  const params = {
    headers: {
      "Content-Type": "application/json",
      "X-Forwarded-For": clientIP,
    },
  };

  const res = http.post(`${BASE_URL}/api/v1/links`, payload, params);

  check(res, {
    "status is 201 Created": (r) => r.status === 201,
    "has short_code in response": (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.data && body.data.short_code && body.data.short_code.length > 0;
      } catch (e) {
        return false;
      }
    },
  });
}
