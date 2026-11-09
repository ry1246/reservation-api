import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const SEAT_ID = Number(__ENV.SEAT_ID);

const created = new Counter('reservations_created');
const conflict = new Counter('reservations_conflict');

export const options = {
  scenarios: {
    race: {
      executor: 'shared-iterations',
      vus: 50,
      iterations: 50,
      maxDuration: '30s',
    },
  },
};

export default function () {
  const payload = JSON.stringify({
    seat_id: SEAT_ID,
    user_name: `user-${__VU}`,
  });
  const params = { headers: { 'Content-Type': 'application/json' } };

  const res = http.post(`${BASE_URL}/reservations`, payload, params);

  if (res.status === 201) {
    created.add(1);
  } else if (res.status === 409) {
    conflict.add(1);
  }

  check(res, {
    'is 201 or 409': (r) => r.status === 201 || r.status === 409,
  });
}
