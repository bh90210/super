import Axios from 'axios';

// NOTE: Spotify's Web API is not used by this fork. The Go/Wails backend owns the
// catalog, so every request is intercepted and answered locally with an empty
// payload. This keeps the upstream service layer (`src/services/*`) and RTK Query
// endpoints intact while the real data source is wired up.
// TODO: replace `getMockResponse` with calls to the Go bindings in `frontend/bindings`.
const path = 'https://api.spotify.com/v1' as const;

const axios = Axios.create({
  baseURL: path,
  headers: {},
});

// Mock all Spotify API calls
axios.interceptors.request.use(
  (config) => {
    // Return mock data instead of making real requests
    return Promise.reject({
      config,
      response: {
        status: 200,
        data: getMockResponse(config.url || '', config.method || 'get'),
      },
    });
  },
  (error) => Promise.reject(error)
);

axios.interceptors.response.use(
  (response) => response,
  (error) => {
    // If it's our mock rejection, return the mock response
    if (error.response?.status === 200) {
      return Promise.resolve(error.response);
    }
    return Promise.reject(error);
  }
);

function getMockResponse(url: string, method: string): any {
  // Return empty/mock data for all Spotify endpoints

  // Browse endpoints
  if (url.includes('/browse/new-releases')) {
    return { albums: { items: [], total: 0, limit: 10, offset: 0 } };
  }

  if (url.includes('/browse/categories') && url.includes('/playlists')) {
    return { playlists: { items: [], total: 0, limit: 10, offset: 0 } };
  }

  if (url.includes('/browse/categories')) {
    return { categories: { items: [], total: 0, limit: 10, offset: 0 } };
  }

  if (url.includes('/browse/featured-playlists')) {
    return { playlists: { items: [], total: 0, limit: 10, offset: 0 } };
  }

  // User endpoints
  if (url.includes('/me/top/tracks')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  if (url.includes('/me/top/artists')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  if (url.includes('/me/playlists')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  if (url.includes('/me/tracks')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  if (url.includes('/me/albums')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  if (url.includes('/me/following')) {
    return { artists: { items: [], total: 0, limit: 10, offset: 0 } };
  }

  if (url.includes('/me/shows')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  if (url.includes('/me/episodes')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  // Player endpoints.
  //
  // Order matters: `/me/player/*` must be matched before the generic `/me/*`
  // handlers below, and the shapes here are dictated by the consumers, not by
  // Spotify. Returning `{ items: [], total: 0 }` for these makes callers throw —
  // e.g. `fetchDevices` reads `response.devices`, and the Spotify slice's
  // `getCurrentDevice` selector then calls `devices.find(...)` on undefined.
  if (url.includes('/me/player/devices')) {
    // `playerService.getAvailableDevices()` -> `{ devices: Device[] }`
    return { devices: [] };
  }

  if (url.includes('/me/player/queue')) {
    // `userService.fetchQueue()` -> `{ currently_playing, queue }`
    return { currently_playing: null, queue: [] };
  }

  if (url.includes('/me/player/recently-played')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  if (url.match(/\/me\/player$/)) {
    // `playerService.fetchPlaybackState()` expects a PlaybackState *or* null;
    // null means "nothing is playing", which is the honest answer here.
    return null;
  }

  // Library-contains must precede the generic `/me/*` handlers: it returns a
  // bare boolean array, not a paginated object.
  if (url.includes('/me/library/contains')) {
    return [];
  }

  if (url.includes('/me/library')) {
    return null;
  }

  if (url.match(/\/me$/)) {
    return {
      id: 'local-user',
      display_name: 'Local User',
      email: 'local@super.app',
      country: 'US',
      product: 'premium',
      followers: { href: null, total: 0 },
      images: [],
      type: 'user',
      uri: 'spotify:user:local-user',
      external_urls: { spotify: '' },
    };
  }

  // Search endpoints
  if (url.includes('/search')) {
    return {
      artists: { items: [], total: 0, limit: 10, offset: 0 },
      albums: { items: [], total: 0, limit: 10, offset: 0 },
      tracks: { items: [], total: 0, limit: 10, offset: 0 },
      playlists: { items: [], total: 0, limit: 10, offset: 0 },
      episodes: { items: [], total: 0, limit: 10, offset: 0 },
      shows: { items: [], total: 0, limit: 10, offset: 0 },
    };
  }

  // Playlist endpoints
  if (url.match(/\/playlists\/[\w]+$/)) {
    return {
      id: 'mock',
      name: 'Mock Playlist',
      description: '',
      images: [],
      tracks: { items: [], total: 0, limit: 10, offset: 0 },
      owner: { display_name: 'Mock User' },
      followers: { href: null, total: 0 },
      type: 'playlist',
      uri: 'spotify:playlist:mock',
    };
  }

  if (url.includes('/playlists') && url.includes('/tracks')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  // Album endpoints
  if (url.match(/\/albums\/[\w]+$/)) {
    return {
      id: 'mock',
      name: 'Mock Album',
      images: [],
      artists: [],
      tracks: { items: [], total: 0, limit: 10, offset: 0 },
      type: 'album',
      uri: 'spotify:album:mock',
    };
  }

  if (url.includes('/albums')) {
    return { albums: [] };
  }

  // Artist endpoints
  if (url.match(/\/artists\/[\w]+\/albums$/)) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  if (url.match(/\/artists\/[\w]+$/)) {
    return {
      id: 'mock',
      name: 'Mock Artist',
      images: [],
      genres: [],
      followers: { total: 0 },
      type: 'artist',
      uri: 'spotify:artist:mock',
    };
  }

  if (url.includes('/artists')) {
    return { artists: [] };
  }

  // Podcast endpoints
  if (url.match(/\/shows\/[\w]+$/)) {
    return {
      id: 'mock',
      name: 'Mock Show',
      images: [],
      publisher: '',
      episodes: { items: [], total: 0, limit: 10, offset: 0 },
      type: 'show',
    };
  }

  if (url.match(/\/episodes\/[\w]+$/)) {
    return {
      id: 'mock',
      name: 'Mock Episode',
      images: [],
      duration_ms: 0,
      type: 'episode',
    };
  }

  if (url.includes('/shows') || url.includes('/episodes')) {
    return { items: [], total: 0, limit: 10, offset: 0 };
  }

  // Default empty response
  return { items: [], total: 0 };
}

export default axios;
