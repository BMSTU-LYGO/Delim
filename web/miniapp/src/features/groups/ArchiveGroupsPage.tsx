import { CellList, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';

import type { Group } from '../../api';
import { userErrorMessage } from '../../api';
import { ErrorState, PageHeader, SkeletonList } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { GroupCard } from './GroupsPage';

export function ArchiveGroupsPage() {
  const { client } = useSession();
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const loadGroups = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(undefined);
    try {
      const page = await client.listGroups({ limit: 50 }, signal);
      setGroups(page.groups.filter((group) => group.status === 'archived'));
    } catch (cause) {
      if (cause instanceof Error && cause.name === 'AbortError') return;
      setError(userErrorMessage(cause, 'Не удалось загрузить архив'));
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    const controller = new AbortController();
    void loadGroups(controller.signal);
    return () => controller.abort();
  }, [loadGroups]);

  return (
    <div className="screen groups-archive">
      <PageHeader title="Архив" />
      {loading ? <SkeletonList /> : null}
      {!loading && error ? <ErrorState description={error} onRetry={() => void loadGroups()} /> : null}
      {!loading && !error && groups.length === 0 ? (
        <Container className="groups-archive__empty">
          <Flex align="center" direction="column" gap={12}>
            <div aria-hidden="true" className="groups-archive__illustration">▱</div>
            <Typography.Headline asChild variant="medium">
              <h3>В архиве пока ничего нет</h3>
            </Typography.Headline>
            <Typography.Body asChild color="secondary" variant="medium">
              <p className="groups-archive__empty-copy">Здесь появятся завершённые и архивированные поездки.</p>
            </Typography.Body>
          </Flex>
        </Container>
      ) : null}
      {!loading && !error && groups.length > 0 ? (
        <Container className="groups-archive__content">
          <CellList className="groups-page__list">
            {groups.map((group) => <GroupCard group={group} key={group.id} />)}
          </CellList>
        </Container>
      ) : null}
    </div>
  );
}
