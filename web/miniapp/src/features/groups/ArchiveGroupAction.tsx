import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useState } from 'react';

import type { Group } from '../../api';
import { userErrorMessage } from '../../api';
import { FormMessage } from '../../components/form';
import { ConfirmDialog } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';

interface ArchiveGroupActionProps {
  group: Group;
  onArchived(group: Group): void;
}

export function ArchiveGroupAction({ group, onArchived }: ArchiveGroupActionProps) {
  const { client } = useSession();
  const [confirming, setConfirming] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();

  if (group.status === 'archived' || group.current_user_role === 'member') return null;

  const archive = async () => {
    if (loading) return;
    setLoading(true);
    setError(undefined);
    try {
      onArchived(await client.archiveGroup(group.id));
    } catch (cause) {
      setError(userErrorMessage(cause, 'Не удалось архивировать группу'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <section aria-labelledby="archive-group-title" className="archive-group-action">
      <Flex direction="column" gap={12}>
        <Flex direction="column" gap={4}>
          <Typography.Headline asChild variant="small">
            <h3 id="archive-group-title">Архив группы</h3>
          </Typography.Headline>
          <Typography.Body color="secondary" variant="small">
            Новые расходы и изменения станут недоступны, но история сохранится.
          </Typography.Body>
        </Flex>
        <Button
          loading={loading}
          onClick={() => setConfirming(true)}
          size="small"
          variant="destructive"
        >
          Архивировать группу
        </Button>
        <FormMessage>{error}</FormMessage>
      </Flex>
      <ConfirmDialog
        confirmLabel="Архивировать"
        description={`В группе «${group.name}» больше нельзя будет добавлять расходы и менять участников.`}
        destructive
        onCancel={() => setConfirming(false)}
        onConfirm={() => void archive()}
        open={confirming}
        title="Архивировать группу?"
      />
    </section>
  );
}
