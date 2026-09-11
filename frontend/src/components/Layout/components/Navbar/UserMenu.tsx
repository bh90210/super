import { memo, useEffect, useMemo, useState } from 'react';

// Components
import { Dropdown, Modal } from 'antd';
import type { MenuProps } from 'antd';
import { WhiteButton } from '../../../Button';

// Icons
import { FaArrowRightToBracket, FaGear, FaRightFromBracket, FaUser } from 'react-icons/fa6';

// Utils
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

// Redux
import { uiActions } from '../../../../store/slices/ui';
import { authActions, loginToSpotify } from '../../../../store/slices/auth';
import { useAppDispatch, useAppSelector } from '../../../../store/store';

// Constants
import { ARTISTS_DEFAULT_IMAGE } from '../../../../constants/spotify';

/**
 * Top-right account menu. Collapses Profile / Settings / Log out behind the
 * avatar when logged in, and offers Log in when logged out.
 */
export const UserMenu = memo(() => {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const { t } = useTranslation(['navbar', 'home']);

  const user = useAppSelector(
    (state) => state.auth.user,
    (prev, next) => prev?.id === next?.id
  );
  const loginButtonOpen = useAppSelector((state) => state.ui.loginButtonOpen);

  const [open, setOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  // Other screens (e.g. saving a song) prompt for login by dispatching
  // `openLoginButton` — surface the menu so the user lands on "Log in".
  useEffect(() => {
    if (loginButtonOpen && !user) {
      setOpen(true);
      dispatch(uiActions.closeLoginButton());
    }
  }, [loginButtonOpen, user, dispatch]);

  const onLogout = () => {
    setConfirmOpen(false);
    dispatch(authActions.logout());
    navigate('/');
  };

  const items = useMemo<MenuProps['items']>(() => {
    if (!user) {
      return [
        {
          key: 'login',
          icon: <FaArrowRightToBracket />,
          label: t('Log In'),
          onClick: () => dispatch(loginToSpotify()),
        },
      ];
    }

    return [
      {
        key: 'profile',
        icon: <FaUser />,
        label: t('Profile'),
        onClick: () => navigate(`/users/${user.id}`),
      },
      {
        key: 'settings',
        icon: <FaGear />,
        label: t('Settings'),
        onClick: () => navigate('/settings'),
      },
      { type: 'divider' },
      {
        key: 'logout',
        icon: <FaRightFromBracket />,
        danger: true,
        label: t('Logout'),
        onClick: () => setConfirmOpen(true),
      },
    ];
  }, [dispatch, navigate, t, user]);

  return (
    <>
      <Dropdown
        open={open}
        onOpenChange={setOpen}
        menu={{ items }}
        placement='bottomRight'
        trigger={['click']}
        rootClassName='user-menu-dropdown'
      >
        <button className='avatar-container' aria-label={t('Account menu')}>
          <img
            className='avatar'
            id='user-avatar'
            alt=''
            src={user?.images?.[0]?.url || ARTISTS_DEFAULT_IMAGE}
          />
        </button>
      </Dropdown>

      <Modal
        centered
        open={confirmOpen}
        footer={null}
        destroyOnHidden
        onCancel={() => setConfirmOpen(false)}
        className='logout-modal'
        title={t('Log out of SUPER?', { ns: 'home' })}
      >
        <p className='logout-modal__description'>
          {t('You will need to log in again to access your library.', { ns: 'home' })}
        </p>

        <div className='logout-modal__actions'>
          <button className='logout-modal__cancel' onClick={() => setConfirmOpen(false)}>
            {t('Cancel', { ns: 'home' })}
          </button>
          <WhiteButton size='small' title={t('Log out', { ns: 'home' })} onClick={onLogout} />
        </div>
      </Modal>
    </>
  );
});

UserMenu.displayName = 'UserMenu';

export default UserMenu;
