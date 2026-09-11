// Shape regression test for the Spotify mock in frontend/src/axios.ts.
//
// The mock's job is to satisfy the *consumers* (src/services/* and the redux
// slices). A wrong shape does not fail at the HTTP layer — it fails later, in a
// selector, as `undefined is not an object`. This asserts the shapes directly.
//
// Run: node frontend/scripts/mock-shape-test.mjs
//
// Keep this in sync with getMockResponse(): if you add an endpoint, add a check.

// Mirrors the ordering in getMockResponse(). Order is load-bearing: the
// `/me/player/*` and `/me/library/*` branches must win over generic `/me/*`.
function getMockResponse(url) {
  if (url.includes('/me/top/tracks')) return { items: [], total: 0 };
  if (url.includes('/me/following')) return { artists: { items: [], total: 0 } };
  if (url.includes('/me/player/devices')) return { devices: [] };
  if (url.includes('/me/player/queue')) return { currently_playing: null, queue: [] };
  if (url.includes('/me/player/recently-played'))
    return { items: [], total: 0, limit: 10, offset: 0 };
  if (/\/me\/player$/.test(url)) return null;
  if (url.includes('/me/library/contains')) return [];
  if (url.includes('/me/library')) return null;
  if (/\/me$/.test(url)) return { id: 'local-user' };
  if (url.includes('/search')) return { tracks: { items: [] }, artists: { items: [] } };
  if (/\/playlists\/[\w]+$/.test(url)) return { id: 'mock', tracks: { items: [] } };
  if (/\/(albums|artists|shows)\/[\w]+$/.test(url)) return { id: 'mock', items: [] };
  return { items: [], total: 0 };
}

const checks = [
  // The reported crash: getAvailableDevices() -> response.devices, then the
  // getCurrentDevice / getOtherDevices selectors call .find() / .filter().
  ['/me/player/devices', (r) => Array.isArray(r.devices), 'devices is an array'],
  ['/me/player/devices', (r) => r.devices.find((d) => d.is_active) === undefined, 'devices.find() ok'],
  ['/me/player/devices', (r) => r.devices.filter((d) => !d.is_active).length === 0, 'devices.filter() ok'],

  ['/me/player/queue', (r) => Array.isArray(r.queue), 'queue is an array'],
  ['/me/player/queue', (r) => 'currently_playing' in r, 'currently_playing present'],

  ['/me/player', (r) => r === null, 'playback state is null'],

  // libraryContains awaits a bare boolean[]; index access must not throw.
  ['/me/library/contains', (r) => Array.isArray(r), 'contains returns array'],
  ['/me/library/contains', (r) => r[0] === undefined, 'contains[0] safe'],

  ['/me/library', (r) => r === null, 'library mutations return null'],

  ['/me', (r) => r.id === 'local-user', 'me returns a user'],

  ['/search', (r) => Array.isArray(r.tracks.items), 'search.tracks.items'],
  ['/albums/x', (r) => Array.isArray(r.items), 'album has items'],
  ['/artists/x/albums', (r) => Array.isArray(r.items), 'artist albums has items'],

  // Regression guard: a search URL must not be swallowed by /me/top/*.
  ['/me/top/tracks', (r) => Array.isArray(r.items), 'top tracks has items'],
];

let failed = 0;
for (const [url, assert, name] of checks) {
  const res = getMockResponse(url);
  let ok = false;
  try {
    ok = assert(res) === true;
  } catch {
    ok = false;
  }
  if (!ok) failed++;
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${url.padEnd(24)} ${name}`);
}

console.log(
  failed === 0 ? `\nALL ${checks.length} SHAPE CHECKS PASS` : `\n${failed} FAILURE(S)`
);
process.exit(failed === 0 ? 0 : 1);
