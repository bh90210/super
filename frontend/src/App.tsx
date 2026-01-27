/* eslint-disable react-hooks/exhaustive-deps */
import './styles/App.scss';

// Utils
import i18next from 'i18next';
import { FC, Suspense, lazy, memo, useCallback, useEffect, useMemo, useRef, Component, ErrorInfo, ReactNode } from 'react';

// Components
import { ConfigProvider } from 'antd';
import { AppLayout } from './components/Layout';
import { Route, BrowserRouter as Router, Routes, useLocation } from 'react-router-dom';

// Redux
import { Provider } from 'react-redux';
import { uiActions } from './store/slices/ui';
import { PersistGate } from 'redux-persist/integration/react';
import { persistor, store, useAppDispatch, useAppSelector } from './store/store';

// Pages
import SearchContainer from './pages/Search/Container';
import { Spinner } from './components/spinner/spinner';

const Home = lazy(() => import('./pages/Home'));
const Page404 = lazy(() => import('./pages/404'));
const AlbumView = lazy(() => import('./pages/Album'));
const GenrePage = lazy(() => import('./pages/Genre'));
const BrowsePage = lazy(() => import('./pages/Browse'));
const ArtistPage = lazy(() => import('./pages/Artist'));
const PlaylistView = lazy(() => import('./pages/Playlist'));
const ArtistDiscographyPage = lazy(() => import('./pages/Discography'));

const Profile = lazy(() => import('./pages/User/Home'));
const ProfileTracks = lazy(() => import('./pages/User/Songs'));
const ProfileArtists = lazy(() => import('./pages/User/Artists'));
const ProfilePlaylists = lazy(() => import('./pages/User/Playlists'));

const SearchPage = lazy(() => import('./pages/Search/Home'));
const SearchTracks = lazy(() => import('./pages/Search/Songs'));
const LikedSongsPage = lazy(() => import('./pages/LikedSongs'));
const SearchAlbums = lazy(() => import('./pages/Search/Albums'));
const SearchPlaylist = lazy(() => import('./pages/Search/Playlists'));
const SearchPageArtists = lazy(() => import('./pages/Search/Artists'));
const RecentlySearched = lazy(() => import('./pages/Search/RecentlySearched'));

window.addEventListener('resize', () => {
  const vh = window.innerWidth;
  if (vh < 950) {
    store.dispatch(uiActions.collapseLibrary());
  }
});

// Error Boundary Component
class ErrorBoundary extends Component<{ children: ReactNode }, { hasError: boolean; error?: Error }> {
  constructor(props: { children: ReactNode }) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Error caught by boundary:', error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{ color: 'white', padding: '20px', backgroundColor: '#1a1a1a' }}>
          <h1>Something went wrong</h1>
          <pre style={{ color: 'red' }}>{this.state.error?.message}</pre>
          <pre>{this.state.error?.stack}</pre>
        </div>
      );
    }

    return this.props.children;
  }
}

const RoutesComponent = memo(() => {
  const location = useLocation();
  const container = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (container.current) {
      container.current.scrollTop = 0;
    }
  }, [location, container]);

  const routes = useMemo(
    () => [
      { path: '', element: <Home container={container} /> },
      { path: '/collection/tracks', element: <LikedSongsPage container={container} /> },
      {
        path: '/playlist/:playlistId',
        element: <PlaylistView container={container} />,
      },
      { path: '/album/:albumId', element: <AlbumView container={container} /> },
      {
        path: '/artist/:artistId/discography',
        element: <ArtistDiscographyPage container={container} />,
      },
      { path: '/artist/:artistId', element: <ArtistPage container={container} /> },
      { path: '/users/:userId/artists', element: <ProfileArtists container={container} /> },
      { path: '/users/:userId/playlists', element: <ProfilePlaylists container={container} /> },
      { path: '/users/:userId/tracks', element: <ProfileTracks container={container} /> },
      { path: '/users/:userId', element: <Profile container={container} /> },
      { path: '/genre/:genreId', element: <GenrePage /> },
      { path: '/search', element: <BrowsePage /> },
      { path: '/recent-searches', element: <RecentlySearched /> },
      {
        path: '/search/:search',
        element: <SearchContainer container={container} />,
        children: [
          {
            path: 'artists',
            element: <SearchPageArtists container={container} />,
          },
          {
            path: 'albums',
            element: <SearchAlbums container={container} />,
          },
          {
            path: 'playlists',
            element: <SearchPlaylist container={container} />,
          },
          {
            path: 'tracks',
            element: <SearchTracks container={container} />,
          },
          {
            path: '',
            element: <SearchPage container={container} />,
          },
        ],
      },
      { path: '*', element: <Page404 /> },
    ],
    [container]
  );

  return (
    <div className='Main-section' ref={container}>
      <div
        style={{
          minHeight: 'calc(100vh - 230px)',
          width: '100%',
        }}
      >
        <Suspense fallback={<div style={{ color: 'white' }}>Loading...</div>}>
          <Routes>
            {routes.map((route) => (
              <Route
                key={route.path}
                path={route.path}
                element={route.element}
              >
                {route?.children
                  ? route.children.map((child) => (
                      <Route
                        key={child.path}
                        path={child.path}
                        element={child.element}
                      />
                    ))
                  : undefined}
              </Route>
            ))}
          </Routes>
        </Suspense>
      </div>
    </div>
  );
});

const RootComponent = () => {
  const language = useAppSelector((state) => state.language.language);

  useEffect(() => {
    document.documentElement.setAttribute('lang', language);
    i18next.changeLanguage(language);
  }, [language]);

  return (
    <Router>
      <AppLayout>
        <RoutesComponent />
      </AppLayout>
    </Router>
  );
};

function App() {
  return (
    <ErrorBoundary>
      <ConfigProvider theme={{ token: { fontFamily: 'SpotifyMixUI' } }}>
        <Provider store={store}>
          <PersistGate loading={null} persistor={persistor}>
            <RootComponent />
          </PersistGate>
        </Provider>
      </ConfigProvider>
    </ErrorBoundary>
  );
}

export default App;
