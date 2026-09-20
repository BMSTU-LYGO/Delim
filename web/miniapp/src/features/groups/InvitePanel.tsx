import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useRef, useState } from 'react';

import type { Invite } from '../../api';
import { userErrorMessage } from '../../api';
import { FormMessage } from '../../components/form';
import { maxBridge } from '../../platform/maxBridge';
import { useSession } from '../../session/SessionProvider';

interface InvitePanelProps {
  groupId: number;
}

const formatExpiry = (value: string) =>
  new Intl.DateTimeFormat('ru-RU', {
    dateStyle: 'long',
    timeStyle: 'short',
  }).format(new Date(value));

export function InvitePanel({ groupId }: InvitePanelProps) {
  const { client } = useSession();
  const [invite, setInvite] = useState<Invite>();
  const [creating, setCreating] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [creationError, setCreationError] = useState<string>();
  const [shareError, setShareError] = useState<string>();
  const [clipboardError, setClipboardError] = useState<string>();
  const [feedback, setFeedback] = useState<string>();
  const inFlight = useRef(false);

  const createInvite = async () => {
    if (inFlight.current) return;
    inFlight.current = true;
    setCreating(true);
    setCreationError(undefined);
    setShareError(undefined);
    setClipboardError(undefined);
    setFeedback(undefined);
    try {
      const createdInvite = await client.createInvite(groupId);
      setInvite(createdInvite);
      if (!createdInvite.deep_link) {
        setCreationError('Приглашение создано, но ссылка MAX недоступна. Проверьте настройку имени бота.');
      }
    } catch (cause) {
      setCreationError(userErrorMessage(cause, 'Не удалось создать приглашение'));
    } finally {
      inFlight.current = false;
      setCreating(false);
    }
  };

  const shareInvite = async () => {
    if (!invite?.deep_link || sharing) return;
    setSharing(true);
    setShareError(undefined);
    setClipboardError(undefined);
    setFeedback(undefined);
    try {
      await maxBridge.share({
        text: 'Присоединяйтесь к нашей группе расходов в «Делим»',
        url: invite.deep_link,
      });
      setFeedback('Окно отправки приглашения открыто');
    } catch (cause) {
      setShareError(userErrorMessage(cause, 'Не удалось открыть отправку в MAX'));
    } finally {
      setSharing(false);
    }
  };

  const copyInvite = async () => {
    if (!invite?.deep_link) return;
    setClipboardError(undefined);
    setFeedback(undefined);
    try {
      await maxBridge.copyText(invite.deep_link);
      setFeedback('Ссылка скопирована');
    } catch (cause) {
      setClipboardError(userErrorMessage(cause, 'Не удалось скопировать ссылку'));
    }
  };

  return (
    <section aria-labelledby="invite-title" className="dashboard-card invite-panel" id="invite">
      <Flex direction="column" gap={12}>
        <Flex direction="column" gap={4}>
          <Typography.Headline asChild variant="small">
            <h3 id="invite-title">Пригласить участников</h3>
          </Typography.Headline>
          <Typography.Body color="secondary" variant="small">
            Отправьте защищённую ссылку в MAX. У приглашения ограниченный срок действия.
          </Typography.Body>
        </Flex>
        <Button
          disabled={creating}
          loading={creating}
          onClick={() => void createInvite()}
          size="small"
          stretched
        >
          {invite ? 'Создать новую ссылку' : 'Создать приглашение'}
        </Button>
        {invite ? (
          <Typography.Body color="tertiary" variant="small">
            Действует до {formatExpiry(invite.expires_at)}
          </Typography.Body>
        ) : null}
        {invite?.deep_link ? (
          <Button
            disabled={creating || sharing}
            loading={sharing}
            onClick={() => void shareInvite()}
            size="small"
            stretched
          >
            Отправить в MAX
          </Button>
        ) : null}
        {invite?.deep_link && shareError ? (
          <Button
            disabled={sharing}
            onClick={() => void copyInvite()}
            size="xsmall"
            variant="ghost"
          >
            Скопировать ссылку
          </Button>
        ) : null}
        <FormMessage>{creationError}</FormMessage>
        <FormMessage>{shareError}</FormMessage>
        <FormMessage>{clipboardError}</FormMessage>
        <FormMessage tone="success">{feedback}</FormMessage>
      </Flex>
    </section>
  );
}
