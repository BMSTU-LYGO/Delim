import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useState } from 'react';

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
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [feedback, setFeedback] = useState<string>();

  const deliver = async (createdInvite: Invite) => {
    if (!createdInvite.deep_link) {
      setFeedback('Код создан, но ссылка MAX недоступна. Проверьте настройку имени бота.');
      return;
    }
    if (maxBridge.getEnvironment().available) {
      await maxBridge.share({
        text: 'Присоединяйтесь к нашей группе расходов в «Делим»',
        url: createdInvite.deep_link,
      });
      setFeedback('Окно отправки приглашения открыто');
      return;
    }
    await maxBridge.copyText(createdInvite.deep_link);
    setFeedback('Ссылка скопирована');
  };

  const createAndShare = async () => {
    if (loading) return;
    setLoading(true);
    setError(undefined);
    setFeedback(undefined);
    try {
      const createdInvite = await client.createInvite(groupId);
      setInvite(createdInvite);
      await deliver(createdInvite);
    } catch (cause) {
      setError(userErrorMessage(cause, 'Не удалось создать приглашение'));
    } finally {
      setLoading(false);
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
        <Button loading={loading} onClick={() => void createAndShare()} size="small" stretched>
          {invite ? 'Создать новую ссылку' : 'Поделиться приглашением'}
        </Button>
        {invite ? (
          <Typography.Body color="tertiary" variant="small">
            Действует до {formatExpiry(invite.expires_at)}
          </Typography.Body>
        ) : null}
        {invite?.deep_link ? (
          <Button
            onClick={() => {
              void maxBridge.copyText(invite.deep_link!).then(() => setFeedback('Ссылка скопирована'));
            }}
            size="xsmall"
            variant="ghost"
          >
            Скопировать ссылку
          </Button>
        ) : null}
        <FormMessage>{error}</FormMessage>
        <FormMessage tone="success">{feedback}</FormMessage>
      </Flex>
    </section>
  );
}
