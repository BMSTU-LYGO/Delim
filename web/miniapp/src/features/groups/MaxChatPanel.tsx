import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';

import { userErrorMessage } from '../../api';
import { maxBridge } from '../../platform/maxBridge';
import { useSession } from '../../session/SessionProvider';

type SubscriptionState =
  | { kind: 'loading' }
  | { kind: 'disconnected'; botURL?: string }
  | { kind: 'connected'; botURL?: string }
  | { kind: 'error'; message: string };

const describe = (state: SubscriptionState): string => {
  switch (state.kind) {
    case 'disconnected':
      return 'Подключите бота, чтобы получать уведомления о тратах и возвратах в личные сообщения MAX.';
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
        setState(result.connected ? { kind: 'connected', botURL: result.bot_url } : { kind: 'disconnected', botURL: result.bot_url });
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
      if (!result.bot_url) throw new Error('MAX bot is not configured');
      maxBridge.openMaxLink(result.bot_url);
    } catch (cause) {
      setState({ kind: 'error', message: userErrorMessage(cause, 'Не удалось открыть диалог с ботом') });
    } finally {
      setBusy(false);
    }
  };

  const disable = async () => {
    setBusy(true);
    try {
      await client.disableMaxSubscription();
      setState({ kind: 'disconnected', botURL: state.kind === 'connected' ? state.botURL : undefined });
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
            <Button data-max-subscription-cta disabled={busy} loading={busy} onClick={() => void connect()} size="small">
              Подключить уведомления
            </Button>
          ) : null}
          {state.kind === 'connected' ? (
            <Button disabled={busy} loading={busy} onClick={() => void disable()} size="small" variant="destructive">
              Отключить
            </Button>
          ) : null}
        </Flex>
      </Flex>
    </section>
  );
}
