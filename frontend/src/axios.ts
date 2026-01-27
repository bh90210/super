import Axios from 'axios';

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
  
  if (url.includes('/me/player')) {
    return { items: [], total: 0 };
  }
  
  // Playlist endpoints
  if (url.match(/\/playlists\/[\w]+$/)) {
    return {
      id: 'mock',
      name: 'Mock Playlist',
      description: '',
      images: [],
      tracks: { items: [], total: 0 },
      owner: { display_name: 'Mock User' },
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
      tracks: { items: [] },
    };
  }
  
  if (url.includes('/albums')) {
    return { albums: [] };
  }
  
  // Artist endpoints
  if (url.match(/\/artists\/[\w]+$/)) {
    return {
      id: 'mock',
      name: 'Mock Artist',
      images: [],
      genres: [],
      followers: { total: 0 },
    };
  }
  
  if (url.includes('/artists')) {
    return { artists: [] };
  }
  
  // Search
  if (url.includes('/search')) {
    return {
      tracks: { items: [], total: 0 },
      artists: { items: [], total: 0 },
      albums: { items: [], total: 0 },
      playlists: { items: [], total: 0 },
    };
  }
  
  // Default empty response
  return { items: [], total: 0 };
}

export default axios;
