import { Button, CellList, CellSimple, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

import type { Group, MemberRole } from '../../api';
import { FormMessage } from '../../components/form';
import { EmptyState, ErrorState, PageHeader, SkeletonList, StatusBadge, UserRow } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

const roleLabels: Record<MemberRole, string> = {
  owner: 'Владелец',
  admin: 'Администратор',
  member: 'Участник',
};

interface GroupCardProps {
  group: Group;
}

function GroupCard({ group }: GroupCardProps) {
  const archived = group.status === 'archived';
  return (
    <CellSimple
      after={
        <StatusBadge tone={archived ? 'neutral' : 'positive'}>
          {archived ? 'В архиве' : 'Активна'}
        </StatusBadge>
      }
      asChild
      className={archived ? 'group-card group-card--archived' : 'group-card'}
      showChevron
      subtitle={roleLabels[group.current_user_role]}
      title={group.name}
    >
      <Link aria-label={`${group.name}, ${roleLabels[group.current_user_role]}`} to={routes.group(String(group.id))} />
    </CellSimple>
  );
}

interface GroupSectionProps {
  groups: Group[];
  title: string;
}

function GroupSection({ groups, title }: GroupSectionProps) {
  if (groups.length === 0) return null;
  return (
    <section aria-labelledby={`groups-${title}`}>
      <Typography.Headline asChild variant="small">
        <h3 className="groups-page__section-title" id={`groups-${title}`}>
          {title}
        </h3>
      </Typography.Headline>
      <CellList className="groups-page__list">
        {groups.map((group) => (
          <GroupCard group={group} key={group.id} />
        ))}
      </CellList>
    </section>
  );
}

export function GroupsPage() {
  const { client, user } = useSession();
  const [groups, setGroups] = useState<Group[]>([]);
  const [cursor, setCursor] = useState<number>();
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string>();

  const loadGroups = useCallback(
    async (nextCursor?: number, signal?: AbortSignal) => {
      if (nextCursor === undefined) setLoading(true);
      else setLoadingMore(true);
      setError(undefined);
      try {
        const page = await client.listGroups({ cursor: nextCursor, limit: 50 }, signal);
        setGroups((current) => {
          if (nextCursor === undefined) return page.groups;
          const known = new Set(current.map((group) => group.id));
          return [...current, ...page.groups.filter((group) => !known.has(group.id))];
        });
        setCursor(page.next_cursor);
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(cause instanceof Error ? cause.message : 'Не удалось загрузить группы');
      } finally {
        setLoading(false);
        setLoadingMore(false);
      }
    },
    [client],
  );

  useEffect(() => {
    const controller = new AbortController();
    void loadGroups(undefined, controller.signal);
    return () => controller.abort();
  }, [loadGroups]);

  const activeGroups = groups.filter((group) => group.status === 'active');
  const archivedGroups = groups.filter((group) => group.status === 'archived');
  const createAction = (
    <Button asChild size="small">
      <Link to={routes.newGroup}>Создать группу</Link>
    </Button>
  );

  return (
    <div className="screen groups-page">
      <PageHeader action={createAction} subtitle="Совместные расходы без ручных расчётов" title="Ваши группы" />
      {user ? (
        <Container>
          <div className="groups-page__profile">
            <UserRow after={<StatusBadge tone="themed">Вы</StatusBadge>} user={user} />
          </div>
        </Container>
      ) : null}

      {loading ? <SkeletonList /> : null}
      {!loading && error && groups.length === 0 ? (
        <ErrorState description={error} onRetry={() => void loadGroups()} />
      ) : null}
      {!loading && !error && groups.length === 0 ? (
        <EmptyState
          action={createAction}
          description="Создайте группу для поездки, дома или встречи — «Делим» посчитает, кто кому должен."
          title="Пока нет групп"
        />
      ) : null}

      {groups.length > 0 ? (
        <Container className="groups-page__sections">
          <Flex direction="column" gap={24}>
            <GroupSection groups={activeGroups} title="Активные" />
            <GroupSection groups={archivedGroups} title="Архив" />
            {error ? <FormMessage>{error}</FormMessage> : null}
            {cursor !== undefined ? (
              <Button
                loading={loadingMore}
                onClick={() => void loadGroups(cursor)}
                size="small"
                stretched
                variant="secondary"
              >
                Показать ещё
              </Button>
            ) : null}
          </Flex>
        </Container>
      ) : null}
    </div>
  );
}
