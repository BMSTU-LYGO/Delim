import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';

import { userErrorMessage } from '../../api';
import { useSession } from '../../session/SessionProvider';

type SubscriptionState =
  | { kind: 'loading' }
  | { kind: 'disconnected' }
  | { kind: 'connected' }
  | { kind: 'error'; message: string };

const describe = (state: SubscriptionState): string => {
  switch (state.kind) {
    case 'disconnected':
      return 'Включите уведомления о тратах и возвратах в этом чате с ботом.';
    case 'connected':
      return 'Уведомления в MAX подключены';
    default:
      return '';
  }
};

export function MaxChatPanel() {
  const { client } = useSession();
  const [state, setState] = useState<SubscriptionState>({ kind: 'loading' });
  const [busy, setBusy] = useState(false);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      setState({ kind: 'loading' });
      try {
        const result = await client.getMaxSubscription(signal);
        setState(result.connected ? { kind: 'connected' } : { kind: 'disconnected' });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setState({ kind: 'error', message: userErrorMessage(cause, 'Не удалось загрузить состояние уведомлений') });
      }
    },
    [client],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const connect = async () => {
    setBusy(true);
    try {
      const result = await client.connectMaxSubscription();
      if (!result.connected) throw new Error('MAX subscription was not connected');
      setState({ kind: 'connected' });
    } catch (cause) {
      setState({ kind: 'error', message: userErrorMessage(cause, 'Не удалось подключить уведомления') });
    } finally {
      setBusy(false);
    }
  };

  const disable = async () => {
    setBusy(true);
    try {
      await client.disableMaxSubscription();
      setState({ kind: 'disconnected' });
    } catch (cause) {
      setState({ kind: 'error', message: userErrorMessage(cause, 'Не удалось отключить уведомления') });
    } finally {
      setBusy(false);
    }
  };

  const note = describe(state);

  return (
    <section aria-labelledby="max-notifications" className="dashboard-card max-subscription" data-max-subscription-state={state.kind}>
      <Typography.Body color="secondary" id="max-notifications" variant="medium">
        Уведомления в MAX
      </Typography.Body>
      <Flex direction="column" gap={10}>
        <Typography.Body variant="small">{note}</Typography.Body>
        {state.kind === 'error' ? <Typography.Body color="negative">{state.message}</Typography.Body> : null}
        <Flex gap={8} wrap="wrap">
          {state.kind === 'disconnected' ? (
            <Button data-max-subscription-cta disabled={busy} loading={busy} onClick={() => void connect()} size="small" stretched>
              Подключить уведомления
            </Button>
          ) : null}
          {state.kind === 'connected' ? (
            <Button disabled={busy} loading={busy} onClick={() => void disable()} size="small" stretched variant="destructive">
              Отключить
            </Button>
          ) : null}
        </Flex>
      </Flex>
    </section>
  );
}
