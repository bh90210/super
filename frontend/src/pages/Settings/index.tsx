import { memo } from 'react';

// Components
import { Col, Row } from 'antd';
import { Link } from 'react-router-dom';

// Utils
import { useTranslation } from 'react-i18next';

// Redux
import { languageActions } from '../../store/slices/language';
import { authActions } from '../../store/slices/auth';
import { useAppDispatch, useAppSelector } from '../../store/store';

// Constants
import { AVAILABLE_LANGUAGES } from '../../constants/languages';

export const Settings = memo(() => {
  const dispatch = useAppDispatch();
  const { t } = useTranslation(['settings', 'navbar']);
  const language = useAppSelector((state) => state.language.language);
  const user = useAppSelector((state) => state.auth.user);
  const current = AVAILABLE_LANGUAGES.find((item) => item.value === language);

  return (
    <div className='Settings-section'>
      <h1 className='Settings-title'>{t('Settings')}</h1>

      <section className='Settings-card'>
        <h2>{t('Account')}</h2>

        {user ? (
          <Row align='middle' justify='space-between' gutter={[16, 16]}>
            <Col>
              <p className='Settings-label'>{t('Signed in as')}</p>
              <p className='Settings-value'>{user.display_name || user.email || user.id}</p>
            </Col>
            <Col>
              <Link className='Settings-link' to={`/users/${user.id}`}>
                {t('View profile')}
              </Link>
            </Col>
          </Row>
        ) : (
          <p className='Settings-value'>{t("You're logged out.")}</p>
        )}
      </section>

      <section className='Settings-card'>
        <h2>{t('Language')}</h2>
        <p className='Settings-label'>
          {t('Currently:')} {current?.englishLabel}
        </p>

        <div className='Settings-languages'>
          {AVAILABLE_LANGUAGES.map((item) => (
            <button
              key={item.value}
              className={item.value === language ? 'selected' : ''}
              onClick={() => dispatch(languageActions.setLanguage({ language: item.value }))}
            >
              <span className='title'>{item.label}</span>
              <span className='subtitle'>{item.englishLabel}</span>
            </button>
          ))}
        </div>
      </section>

      {user ? (
        <section className='Settings-card'>
          <h2>{t('Session')}</h2>
          <button className='Settings-logout' onClick={() => dispatch(authActions.logout())}>
            {t('Logout', { ns: 'navbar' })}
          </button>
        </section>
      ) : null}
    </div>
  );
});

Settings.displayName = 'Settings';

export default Settings;
