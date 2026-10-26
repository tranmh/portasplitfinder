import http from 'k6/http';
import { check } from 'k6';

const API = __ENV.AQAA_APP_API_URL || 'http://localhost:8080';
const WEB = __ENV.AQAA_APP_WEB_URL || 'http://localhost:3000';
const VUS = parseInt(__ENV.AQAA_LOAD_VUS || '10', 10);
const DURATION = __ENV.AQAA_LOAD_DURATION || '30s';

export const options = {
  scenarios: {
    load: { executor: 'constant-vus', vus: Math.min(VUS, 10), duration: DURATION },
  },
};

const PLZS = ['73035', '70173', '10115', '80331', '50667'];

export function setup() {
  const terms = ['a', 'e', 'markt', 'Stuttgart'];
  const res = http.get(`${API}/api/stores?pageSize=100`, { tags: { endpoint: 'setup' } });
  if (res.status === 200) {
    try {
      for (const s of (res.json('stores') || []).slice(0, 20)) {
        const st = s.store || s;
        if (st.city) terms.push(st.city);
        if (st.name) terms.push(String(st.name).slice(0, 5));
      }
    } catch (e) {
      // keep the fallback terms
    }
  }
  let productIds = [];
  const pr = http.get(`${API}/api/products`, { tags: { endpoint: 'setup' } });
  if (pr.status === 200) {
    try {
      productIds = (pr.json('products') || []).map((p) => p.id).filter(Boolean);
    } catch (e) {
      productIds = [];
    }
  }
  return { terms, productIds };
}

function pick(a) {
  return a[Math.floor(Math.random() * a.length)];
}

function get(name, url) {
  const res = http.get(url, { tags: { endpoint: name } });
  check(res, { 'status is 200': (r) => r.status === 200 }, { endpoint: name });
}

const endpoints = [
  {
    name: 'api GET /api/stores',
    weight: 30,
    run: (d) => {
      const params = [];
      const r = Math.random();
      if (r < 0.7) params.push(`q=${encodeURIComponent(pick(d.terms))}`);
      if (Math.random() < 0.5) params.push(`plz=${pick(PLZS)}`);
      params.push(`pageSize=${pick([20, 50, 100])}`);
      get('api GET /api/stores', `${API}/api/stores?${params.join('&')}`);
    },
  },
  {
    name: 'api GET /api/availability',
    weight: 25,
    run: (d) => {
      const params = [`plz=${pick(PLZS)}`, `radius=${pick([10, 50, 200])}`];
      if (d.productIds.length && Math.random() < 0.5) params.push(`productId=${encodeURIComponent(pick(d.productIds))}`);
      get('api GET /api/availability', `${API}/api/availability?${params.join('&')}`);
    },
  },
  { name: 'api GET /api/products', weight: 8, run: () => get('api GET /api/products', `${API}/api/products`) },
  { name: 'api GET /api/sources', weight: 8, run: () => get('api GET /api/sources', `${API}/api/sources`) },
  { name: 'api GET /api/health', weight: 4, run: () => get('api GET /api/health', `${API}/api/health`) },
  { name: 'web GET /', weight: 10, run: () => get('web GET /', `${WEB}/`) },
  { name: 'web GET /api/health', weight: 5, run: () => get('web GET /api/health', `${WEB}/api/health`) },
];

const TOTAL = endpoints.reduce((s, e) => s + e.weight, 0);

export default function (data) {
  let r = Math.random() * TOTAL;
  for (const e of endpoints) {
    r -= e.weight;
    if (r < 0) {
      e.run(data);
      return;
    }
  }
  endpoints[0].run(data);
}
