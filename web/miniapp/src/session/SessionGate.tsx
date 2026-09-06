import { Button, Container, Flex, Panel, Spinner, Typography } from '@maxhub/max-ui';
import { useEffect, useRef } from 'react';

import { App } from '../app/App';
import { useSession } from './SessionProvider';

export function SessionGate() {
  const { error, retry, status } = useSession();
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    void retry();
  }, [retry]);

  if (status === 'authenticated') return <App />;

  const loading = status === 'initializing';
  return (
    <Panel centeredX centeredY className="session-gate" mode="secondary">
      <Container>
        <Flex align="center" direction="column" gap={16}>
          {loading ? <Spinner aria-label="Загрузка" size={32} /> : null}
          <Typography.Headline asChild variant="large-strong">
            <h1>{loading ? 'Подключаем «Делим»' : 'Не удалось подключиться'}</h1>
          </Typography.Headline>
          <Typography.Body asChild color="secondary">
            <p aria-live="polite">
              {loading ? 'Проверяем сессию и загружаем данные…' : error}
            </p>
          </Typography.Body>
          {!loading ? (
            <Button onClick={() => void retry()} size="medium">
              Повторить
            </Button>
          ) : null}
        </Flex>
      </Container>
    </Panel>
  );
}
