import { Button, Container, Flex, Panel, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

import { maxBridge } from '../platform/maxBridge';

const pageTitles: Array<[RegExp, string]> = [
  [/^\/$/, 'Ваши группы'],
  [/^\/groups\/new$/, 'Новая группа'],
  [/^\/groups\/[^/]+\/expense\/new$/, 'Новый расход'],
  [/^\/groups\/[^/]+\/balance$/, 'Баланс'],
  [/^\/groups\/[^/]+\/members$/, 'Участники'],
  [/^\/groups\/[^/]+\/settlements$/, 'Взаиморасчёты'],
  [/^\/groups\/[^/]+$/, 'Группа'],
  [/^\/expenses\/[^/]+\/edit$/, 'Редактирование расхода'],
  [/^\/expenses\/[^/]+$/, 'Расход'],
  [/^\/receipts\/[^/]+$/, 'Чек'],
];

const getPageTitle = (pathname: string) =>
  pageTitles.find(([pattern]) => pattern.test(pathname))?.[1] ?? 'Делим';

export function AppShell() {
  const location = useLocation();
  const navigate = useNavigate();
  const isRoot = location.pathname === '/';

  const goBack = useCallback(() => {
    const historyIndex = window.history.state?.idx as number | undefined;
    if (historyIndex && historyIndex > 0) {
      navigate(-1);
      return;
    }
    navigate('/', { replace: true });
  }, [navigate]);

  useEffect(() => {
    if (isRoot) {
      maxBridge.hideBackButton();
      return;
    }
    return maxBridge.onBack(goBack);
  }, [goBack, isRoot]);

  useEffect(() => {
    const syncViewport = () => {
      const height = window.visualViewport?.height ?? window.innerHeight;
      document.documentElement.style.setProperty('--app-viewport-height', `${height}px`);
    };
    syncViewport();
    window.addEventListener('resize', syncViewport);
    window.visualViewport?.addEventListener('resize', syncViewport);
    return () => {
      window.removeEventListener('resize', syncViewport);
      window.visualViewport?.removeEventListener('resize', syncViewport);
      document.documentElement.style.removeProperty('--app-viewport-height');
    };
  }, []);

  return (
    <Panel className="app" mode="secondary">
      <header className="app-header">
        <Container>
          <Flex align="center" gap={8}>
            {!isRoot && !maxBridge.getEnvironment().available ? (
              <Button
                aria-label="Назад"
                className="app-header__back"
                onClick={goBack}
                size="small"
                variant="ghost"
              >
                ←
              </Button>
            ) : null}
            <Typography.Headline asChild variant="medium">
              <h1>{getPageTitle(location.pathname)}</h1>
            </Typography.Headline>
          </Flex>
        </Container>
      </header>
      <main className="app-content">
        <Outlet />
      </main>
    </Panel>
  );
}
