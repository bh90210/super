import { createAsyncThunk, createSlice, PayloadAction } from '@reduxjs/toolkit';

// Utils
import axios from '../../axios';
import login from '../../utils/spotify/login';

// Services
import { authService } from '../../services/auth';

// Interfaces
import type { User } from '../../interfaces/user';
import { getFromLocalStorageWithExpiry } from '../../utils/localstorage';

// Mock user for bypassing Spotify auth
const mockUser: User = {
  id: 'local-user',
  display_name: 'Local User',
  email: 'local@super.app',
  images: [
    { url: '', height: 0, width: 0 },
    { url: '', height: 0, width: 0 },
  ],
  country: 'US',
  followers: { href: null, total: 0 },
  product: 'premium',
  type: 'user',
  uri: 'spotify:user:local-user',
  external_urls: { spotify: '' },
};

const initialState: { token?: string; playerLoaded: boolean; user?: User; requesting: boolean } = {
  user: mockUser, // Set mock user by default
  requesting: false, // Not requesting since we're bypassing auth
  playerLoaded: true, // Set to true to skip player checks
  token: 'mock-token',
};

export const loginToSpotify = createAsyncThunk<{ token?: string; loaded: boolean }>(
  'auth/loginToSpotify',
  async (_, thunkAPI) => {
    // Bypassed - just return mock data
    return { token: 'mock-token', loaded: true };
  }
);

export const fetchUser = createAsyncThunk('auth/fetchUser', async () => {
  // Return mock user instead of fetching from Spotify
  return mockUser;
});

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    setRequesting(state, action: PayloadAction<{ requesting: boolean }>) {
      state.requesting = action.payload.requesting;
    },
    setToken(state, action: PayloadAction<{ token?: string }>) {
      state.token = action.payload.token;
    },
    setPlayerLoaded(state, action: PayloadAction<{ playerLoaded: boolean }>) {
      state.playerLoaded = action.payload.playerLoaded;
    },
  },
  extraReducers: (builder) => {
    builder.addCase(loginToSpotify.fulfilled, (state, action) => {
      state.token = action.payload.token;
      state.requesting = !action.payload.loaded;
    });
    builder.addCase(fetchUser.fulfilled, (state, action) => {
      state.user = action.payload;
      state.requesting = false;
    });
  },
});

export const authActions = { ...authSlice.actions, loginToSpotify, fetchUser };

export default authSlice.reducer;
