import { Button, CellList, CellSimple, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

import type { BalanceBreakdown, Group, GroupMember, User } from '../../api';
import { userErrorMessage } from '../../api';
import { EmptyState, ErrorState, Money, PageHeader, SkeletonList } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

const operationLabels: Record<string, string> = {
  expense: 'Оплата расхода',
  allocation: 'Доля в расходе',
  settlement_sent: 'Отправленное погашение',
  settlement_received: 'Полученное погашение',
  adjustment_payer: 'Возврат или корректировка плательщику',
  adjustment_allocation: 'Доля возврата или корректировки',
};

interface BreakdownData {
  breakdown: BalanceBreakdown;
  group: Group;
  member?: GroupMember;
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

const formatDate = (value: string) =>
  new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(value),
  );

interface BalanceBreakdownPageProps {
  groupId: number;
  userId: number;
}

export function BalanceBreakdownPage({ groupId, userId }: BalanceBreakdownPageProps) {
  const { client } = useSession();
  const [data, setData] = useState<BreakdownData>();
  const [error, setError] = useState<string>();

  const load = useCallback(
    async (signal?: AbortSignal) => {
      setError(undefined);
      try {
        const [group, members, breakdown] = await Promise.all([
          client.getGroup(groupId, signal),
          client.listGroupMembers(groupId, signal),
          client.getBalanceBreakdown(groupId, userId, signal),
        ]);
        setData({ breakdown, group, member: members.find((member) => member.user_id === userId) });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(userErrorMessage(cause, 'Не удалось загрузить детализацию'));
      }
    },
    [client, groupId, userId],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  if (error && !data) return <ErrorState description={error} onRetry={() => void load()} />;
  if (!data) return <SkeletonList count={6} />;

  return (
    <div className="screen balance-breakdown">
      <PageHeader
        action={
          <Button asChild size="xsmall" variant="ghost">
            <Link to={routes.balance(String(groupId))}>К списку</Link>
          </Button>
        }
        subtitle={data.group.name}
        title={userName(data.member)}
      />
      <Container>
        <Flex direction="column" gap={20}>
          <section className="balance-breakdown__summary">
            <Typography.Body color="secondary" variant="small">
              Итоговый баланс
            </Typography.Body>
            {data.breakdown.balance.length ? (
              data.breakdown.balance.map((balance) => (
                <Typography.Headline asChild key={balance.currency} variant="large-strong">
                  <Money
                    amountMinor={balance.net_amount_minor}
                    className={balance.net_amount_minor < 0 ? 'balance-negative' : 'balance-positive'}
                    currency={balance.currency}
                    sign="always"
                  />
                </Typography.Headline>
              ))
            ) : (
              <Typography.Headline variant="large-strong">Расчёты закрыты</Typography.Headline>
            )}
          </section>

          <section aria-labelledby="balance-operations-title">
            <Typography.Headline asChild variant="small">
              <h3 id="balance-operations-title">Операции</h3>
            </Typography.Headline>
            {data.breakdown.entries.length ? (
              <CellList className="balance-page__list">
                {data.breakdown.entries.map((entry, index) => (
                  <CellSimple
                    after={
                      <Money
                        amountMinor={entry.amount_minor}
                        className={entry.amount_minor < 0 ? 'balance-negative' : 'balance-positive'}
                        currency={entry.currency}
                        sign="always"
                      />
                    }
                    key={`${entry.operation_type}-${entry.operation_id}-${index}`}
                    subtitle={`${formatDate(entry.occurred_at)} · #${entry.operation_id}`}
                    title={operationLabels[entry.operation_type] ?? 'Операция баланса'}
                  />
                ))}
              </CellList>
            ) : (
              <EmptyState
                description="Подтверждённые расходы, погашения и возвраты появятся здесь."
                title="Операций пока нет"
              />
            )}
          </section>
        </Flex>
      </Container>
    </div>
  );
}
