import { memo } from 'react';

import { Space } from 'antd';
import { UserMenu } from './UserMenu';

// Utils
import { useTranslation } from 'react-i18next';

// Constants
import useIsMobile from '../../../../utils/isMobile';

const Header = memo(({ opacity }: { opacity: number; title?: string }) => {
  const isMobile = useIsMobile();
  const { t } = useTranslation(['navbar']);

  return (
    <div
      className={`flex r-0 w-full flex-row items-center justify-between bg-gray-900 rounded-t-md z-10`}
      style={{ backgroundColor: `rgba(12, 12, 12, ${opacity}%)` }}
    >
      <div className='flex flex-row items-center'>
        <Space>
          {!isMobile ? (
            <a
              target='_blank'
              rel='noreferrer'
              className='contact-me'
              href='https://github.com/francoborrelli/spotify-react-web-client'
            >
              <span>{t('Source code')}</span>
            </a>
          ) : null}

          {/*
          <div className='news'>
            <News />
          </div> */}

          {/* Account menu: avatar dropdown with Profile / Settings / Log out */}
          <UserMenu />
        </Space>
      </div>
    </div>
  );
});

Header.displayName = 'Header';

export default Header;
