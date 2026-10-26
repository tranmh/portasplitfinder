import http from 'k6/http';
import { check } from 'k6';

const API = __ENV.AQAA_APP_API_URL || 'http://localhost:8080';
const WEB = __ENV.AQAA_APP_WEB_URL || 'http://localhost:3000';

export const options = {
  scenarios: {
    load: {
      executor: 'constant-vus',
      vus: parseInt(__ENV.AQAA_LOAD_VUS || '10', 10),
      duration: __ENV.AQAA_LOAD_DURATION || '30s',
    },
  },
};

export function setup() {
  const res = http.get(`${API}/api/products`, { tags: { endpoint: 'api GET /api/products' } });
  let ids = [];
  try {
    ids = (res.json('products') || []).map((p) => p.id).filter(Boolean);
  } catch (e) {
    ids = [];
  }
  return { productIds: ids };
}

function get(name, url, params) {
  return http.get(url, Object.assign({ tags: { endpoint: name } }, params || {}));
}

const ok200 = { 'status is 200': (r) => r.status === 200 };
const notFound = { 'status is 404': (r) => r.status === 404 };
const expect404 = http.expectedStatuses(404);

const endpoints = [
  {
    // P2/P5: DISTINCT ON over all snapshots + stores + sources per request.
    name: 'api GET /api/availability',
    weight: 45,
    run: (data) => {
      const variants = ['', '?plz=73035&radius=50', '?plz=10115&radius=100'];
      let q = variants[Math.floor(Math.random() * variants.length)];
      const ids = data.productIds;
      if (ids.length > 0 && Math.random() < 0.5) {
        const pid = ids[Math.floor(Math.random() * ids.length)];
        q += (q ? '&' : '?') + 'productId=' + encodeURIComponent(pid);
      }
      return [get('api GET /api/availability', `${API}/api/availability${q}`), ok200];
    },
  },
  { name: 'api GET /api/products', weight: 15, run: () => [get('api GET /api/products', `${API}/api/products`), ok200] },
  { name: 'api GET /api/sources', weight: 12, run: () => [get('api GET /api/sources', `${API}/api/sources`), ok200] },
  { name: 'api GET /api/health', weight: 8, run: () => [get('api GET /api/health', `${API}/api/health`), ok200] },
  {
    name: 'api GET /api/subscriptions/confirm',
    weight: 4,
    run: () => [
      get('api GET /api/subscriptions/confirm', `${API}/api/subscriptions/confirm?token=aqaa-nonexistent-${__VU}-${__ITER}`, { responseCallback: expect404 }),
      notFound,
    ],
  },
  {
    name: 'api GET /api/subscriptions/unsubscribe',
    weight: 4,
    run: () => [
      get('api GET /api/subscriptions/unsubscribe', `${API}/api/subscriptions/unsubscribe?token=aqaa-nonexistent-${__VU}-${__ITER}`, { responseCallback: expect404 }),
      notFound,
    ],
  },
  { name: 'web GET /', weight: 8, run: () => [get('web GET /', `${WEB}/`), ok200] },
  { name: 'web GET /api/health', weight: 4, run: () => [get('web GET /api/health', `${WEB}/api/health`), ok200] },
];

const total = endpoints.reduce((s, e) => s + e.weight, 0);

export default function (data) {
  let r = Math.random() * total;
  let ep = endpoints[endpoints.length - 1];
  for (const e of endpoints) {
    if (r < e.weight) {
      ep = e;
      break;
    }
    r -= e.weight;
  }
  const [res, checks] = ep.run(data);
  check(res, checks, { endpoint: ep.name });
}
