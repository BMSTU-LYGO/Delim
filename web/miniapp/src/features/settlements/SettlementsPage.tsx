import { Button, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';

import type { Group, GroupMember, SettlementPlanTransfer, User } from '../../api';
import { EmptyState, ErrorState, Money, PageHeader, SkeletonList, StatusBadge } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

interface SettlementPlanData {
  group: Group;
  members: GroupMember[];
  plan: SettlementPlanTransfer[];
}

const userFor = (member?: GroupMember): User | undefined =>
  member?.user ??
  (member
    ? {
        first_name: '',
        id: member.user_id,
        last_name: '',
        max_user_id: 0,
        username: `id${member.user_id}`,
      }
    : undefined);

const userName = (member?: GroupMember) => {
  const user = userFor(member);
  if (!user) return 'Участник';
  return [user.first_name, user.last_name].filter(Boolean).join(' ') || user.username;
};

const settlementQuery = (transfer: SettlementPlanTransfer) => {
  const query = new URLSearchParams({
    amount: String(transfer.amount_minor),
    currency: transfer.currency,
    from: String(transfer.from_user_id),
    to: String(transfer.to_user_id),
  });
  return query.toString();
};

export function SettlementsPage() {
  const { groupId } = useParams();
  const numericGroupId = Number(groupId);
  const { client, user } = useSession();
  const [data, setData] = useState<SettlementPlanData>();
  const [error, setError] = useState<string>();

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!Number.isSafeInteger(numericGroupId) || numericGroupId <= 0) {
        setError('Некорректный идентификатор группы');
        return;
      }
      setError(undefined);
      try {
        const [group, members, plan] = await Promise.all([
          client.getGroup(numericGroupId, signal),
          client.listGroupMembers(numericGroupId, signal),
          client.getSettlementPlan(numericGroupId, signal),
        ]);
        setData({ group, members, plan });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(cause instanceof Error ? cause.message : 'Не удалось загрузить план погашений');
      }
    },
    [client, numericGroupId],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  if (error && !data) return <ErrorState description={error} onRetry={() => void load()} />;
  if (!data) return <SkeletonList count={5} />;

  const memberById = new Map(data.members.map((member) => [member.user_id, member]));

  return (
    <div className="screen settlements-page">
      <PageHeader subtitle={data.group.name} title="Погашения" />
      <Container>
        <Flex direction="column" gap={20}>
          <div className="settlements-page__notice">
            <Typography.Body color="secondary" variant="small">
              «Делим» предлагает, кому и сколько перевести, и фиксирует расчёт. Деньги нужно
              отправить отдельно удобным вам способом.
            </Typography.Body>
          </div>

          <section aria-labelledby="settlement-plan-title">
            <Typography.Headline asChild variant="small">
              <h3 id="settlement-plan-title">Предложенный план</h3>
            </Typography.Headline>
            {data.plan.length ? (
              <div className="settlement-plan">
                {data.plan.map((transfer) => {
                  const sender = userName(memberById.get(transfer.from_user_id));
                  const receiver = userName(memberById.get(transfer.to_user_id));
                  const currentSends = transfer.from_user_id === user?.id;
                  const currentReceives = transfer.to_user_id === user?.id;
                  return (
                    <article
                      className="settlement-transfer"
                      key={`${transfer.from_user_id}-${transfer.to_user_id}-${transfer.currency}`}
                    >
                      <Flex align="center" gap={8} justify="space-between">
                        <div>
                          <Typography.Body asChild variant="large-strong">
                            <h4>
                              {sender} → {receiver}
                            </h4>
                          </Typography.Body>
                          {currentSends ? <StatusBadge tone="warning">Вы платите</StatusBadge> : null}
                          {currentReceives ? (
                            <StatusBadge tone="positive">Вы получите</StatusBadge>
                          ) : null}
                        </div>
                        <Typography.Headline asChild variant="small">
                          <Money amountMinor={transfer.amount_minor} currency={transfer.currency} />
                        </Typography.Headline>
                      </Flex>
                      {currentSends && data.group.status === 'active' ? (
                        <Button asChild size="small" stretched>
                          <Link
                            to={`${routes.settlements(String(data.group.id))}?${settlementQuery(transfer)}`}
                          >
                            Отметить погашение
                          </Link>
                        </Button>
                      ) : null}
                    </article>
                  );
                })}
              </div>
            ) : (
              <EmptyState
                description="Никому не нужно переводить деньги по текущим подтверждённым операциям."
                title="Расчёты закрыты"
              />
            )}
          </section>
        </Flex>
      </Container>
    </div>
  );
}
