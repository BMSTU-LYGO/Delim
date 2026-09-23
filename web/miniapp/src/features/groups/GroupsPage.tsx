import { Button, CellList, CellSimple, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

import type { Group, MemberRole } from '../../api';
import { userErrorMessage } from '../../api';
import { FormMessage } from '../../components/form';
import { EmptyState, ErrorState, PageHeader, SkeletonList, StatusBadge, UserRow } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

const roleLabels: Record<MemberRole, string> = {
  owner: 'Владелец',
  admin: 'Администратор',
  member: 'Участник',
};

const activityLabels = {
  trip: 'Поездка',
  hike: 'Поход',
  event: 'Событие',
} as const;

const activitySummary = (group: Group) => {
  const parts = [
    group.activity_type ? activityLabels[group.activity_type] : undefined,
    group.location || undefined,
  ].filter(Boolean);
  return parts.join(' · ');
};

interface GroupCardProps {
  group: Group;
}

export function GroupCard({ group }: GroupCardProps) {
  const archived = group.status === 'archived';
  const activity = activitySummary(group);
  return (
    <CellSimple
      after={
        <StatusBadge tone={archived ? 'neutral' : 'positive'}>
          {archived ? 'В архиве' : 'Активна'}
        </StatusBadge>
      }
      asChild
      className={archived ? 'group-card group-card--archived' : 'group-card'}
      innerClassNames={{
        content: 'group-card__content',
        subtitle: 'group-card__subtitle',
        title: 'group-card__title',
      }}
      showChevron
      subtitle={[activity, roleLabels[group.current_user_role]].filter(Boolean).join(' · ')}
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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [showAllActive, setShowAllActive] = useState(false);

  const loadGroups = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true);
      setError(undefined);
      try {
        const page = await client.listGroups({ limit: 50 }, signal);
        setGroups(page.groups);
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(userErrorMessage(cause, 'Не удалось загрузить группы'));
      } finally {
        setLoading(false);
      }
    },
    [client],
  );

  useEffect(() => {
    const controller = new AbortController();
    void loadGroups(controller.signal);
    return () => controller.abort();
  }, [loadGroups]);

  const activeGroups = groups.filter((group) => group.status === 'active');
  const visibleActiveGroups = showAllActive ? activeGroups : activeGroups.slice(0, 3);
  const createAction = (
    <Button asChild size="small">
      <Link to={routes.newGroup}>Создать группу</Link>
    </Button>
  );

  return (
    <div className="screen groups-page">
      {user ? (
        <Container>
          <div className="groups-page__profile">
            <UserRow after={<StatusBadge tone="themed">Вы</StatusBadge>} user={user} />
          </div>
        </Container>
      ) : null}
      <PageHeader
        action={createAction}
        subtitle="Собирайте друзей на выходные и сразу фиксируйте общие траты"
        title="Ваши планы"
      />

      {loading ? <SkeletonList /> : null}
      {!loading && error && groups.length === 0 ? (
        <ErrorState description={error} onRetry={() => void loadGroups()} />
      ) : null}
      {!loading && !error && groups.length === 0 ? (
        <EmptyState
          action={createAction}
          description="Создайте план для поездки, концерта или ужина. Пригласите друзей, добавьте траты — «Делим» рассчитает долги."
          title="Запланируем что-нибудь?"
        />
      ) : null}

      {groups.length > 0 ? (
        <Container className="groups-page__sections">
          <Flex direction="column" gap={24}>
            <div className="groups-page__active">
              <GroupSection groups={visibleActiveGroups} title="Активные" />
              {activeGroups.length > 3 && !showAllActive ? (
                <Button onClick={() => setShowAllActive(true)} size="small" stretched variant="secondary">
                  Показать ещё
                </Button>
              ) : null}
            </div>
            <Button asChild size="small" stretched variant="secondary">
              <Link to={routes.archive}>Архив</Link>
            </Button>
            {error ? <FormMessage>{error}</FormMessage> : null}
          </Flex>
        </Container>
      ) : null}
    </div>
  );
}
