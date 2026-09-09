import { Button, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';

import type { MAXChatBinding, MAXChatBindingState } from '../../api';
import { ApiError, userErrorMessage } from '../../api';
import { useSession } from '../../session/SessionProvider';

type ChatState =
  | { kind: 'loading' }
  | { kind: 'unbound'; roleCanManage: boolean }
  | { kind: 'bound'; binding: MAXChatBinding; roleCanManage: boolean }
  | { kind: 'error'; message: string };

const describe = (state: ChatState, inChat: boolean): { title: string; note: string } => {
  switch (state.kind) {
    case 'unbound':
      if (!inChat) return { title: 'MAX-чат не привязан', note: 'Откройте Делим из группового чата, чтобы привязать его к группе.' };
      return { title: 'Можно привязать текущий чат', note: 'Привяжите этот MAX-чат к группе, чтобы бот помогал вести расходы.' };
    case 'bound':
      if (!state.binding.chat_active) return { title: 'Бот удалён из чата', note: 'Добавьте бота обратно в привязанный чат, чтобы синхронизация работала.' };
      if (!state.roleCanManage) return { title: 'MAX-чат привязан', note: 'Управлять чатом может только владелец или администратор группы.' };
      return { title: 'MAX-чат привязан', note: 'Бот будет присылать уведомления о расходах и погашениях.' };
    default:
      return { title: 'MAX-чат', note: '' };
  }
};

export function MaxChatPanel({ groupId }: { groupId: number }) {
  const { client, user } = useSession();
  const [state, setState] = useState<ChatState>({ kind: 'loading' });
  const [busy, setBusy] = useState(false);
  const [syncNote, setSyncNote] = useState<string>();
  const inChat = Boolean(user?.max_chat_id);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      setState({ kind: 'loading' });
      try {
        const result = await client.getMaxChat(groupId, signal);
        if ('bound' in result && !result.bound) {
          setState({ kind: 'unbound', roleCanManage: result.role_can_manage });
        } else if ('chat_id' in result) {
          setState({ kind: 'bound', binding: result as MAXChatBinding, roleCanManage: true });
        } else {
          setState({ kind: 'unbound', roleCanManage: true });
        }
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setState({ kind: 'error', message: userErrorMessage(cause, 'Не удалось загрузить MAX-чат') });
      }
    },
    [client, groupId],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const bind = async () => {
    setBusy(true);
    try {
      const binding = await client.bindMaxChat(groupId);
      setState({ kind: 'bound', binding, roleCanManage: true });
    } catch (cause) {
      setState({ kind: 'error', message: userErrorMessage(cause, 'Не удалось привязать чат') });
    } finally {
      setBusy(false);
    }
  };

  const unbind = async () => {
    setBusy(true);
    try {
      await client.unbindMaxChat(groupId);
      setState({ kind: 'unbound', roleCanManage: true });
    } catch (cause) {
      setState({ kind: 'error', message: userErrorMessage(cause, 'Не удалось отвязать чат') });
    } finally {
      setBusy(false);
    }
  };

  const sync = async () => {
    setBusy(true);
    setSyncNote(undefined);
    try {
      const result = await client.syncMaxChat(groupId);
      setSyncNote(`Добавлено: ${result.added}, уже в группе: ${result.already_present}, недоступно: ${result.unavailable}.`);
    } catch (cause) {
      if (cause instanceof ApiError && cause.code === 'bot_admin_required') {
        setSyncNote('Для синхронизации участников бот должен быть администратором чата.');
      } else {
        setSyncNote(userErrorMessage(cause, 'Синхронизация недоступна'));
      }
    } finally {
      setBusy(false);
    }
  };

  const canManage = state.kind !== 'loading' && state.kind !== 'error' && state.roleCanManage;
  const { title, note } = describe(state, inChat);

  return (
    <section aria-labelledby="max-chat" className="dashboard-card" data-max-chat-state={state.kind}>
      <Typography.Body color="secondary" id="max-chat" variant="medium">
        MAX-чат
      </Typography.Body>
      <Flex direction="column" gap={10}>
        <Typography.Body variant="small">{note}</Typography.Body>
        {state.kind === 'error' ? <Typography.Body color="negative">{state.message}</Typography.Body> : null}
        {state.kind === 'bound' ? (
          <Typography.Body color="tertiary" variant="small">
            {title}
          </Typography.Body>
        ) : null}
        {syncNote ? (
          <Typography.Body color="tertiary" variant="small" role="status">
            {syncNote}
          </Typography.Body>
        ) : null}
        <Container>
          <Flex gap={8} wrap="wrap">
            {state.kind === 'unbound' && canManage ? (
              <Button disabled={busy || !inChat} loading={busy} onClick={() => void bind()} size="small">
                Привязать
              </Button>
            ) : null}
            {state.kind === 'bound' && canManage ? (
              <>
                <Button disabled={busy} loading={busy} onClick={() => void sync()} size="small" variant="secondary">
                  Синхронизировать участников
                </Button>
                <Button disabled={busy} onClick={() => void unbind()} size="small" variant="destructive">
                  Отвязать
                </Button>
              </>
            ) : null}
          </Flex>
        </Container>
      </Flex>
    </section>
  );
}