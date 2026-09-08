import { Button, CellList, CellSimple, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';

import type { Balance, Group, GroupMember, User } from '../../api';
import { userErrorMessage } from '../../api';
import { EmptyState, ErrorState, Money, PageHeader, SkeletonList, UserAvatar } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

interface BalanceData {
  balances: Balance[];
  group: Group;
  members: GroupMember[];
}

const userFor = (member: GroupMember): User =>
  member.user ?? {
    first_name: '',
    id: member.user_id,
    last_name: '',
    max_user_id: 0,
    username: 'Участник',
  };

const userName = (member: GroupMember) => {
  const user = userFor(member);
  return [user.first_name, user.last_name].filter(Boolean).join(' ') || user.username;
};

interface BalancePhraseProps {
  balance: Balance;
  current: boolean;
}

function BalancePhrase({ balance, current }: BalancePhraseProps) {
  if (balance.net_amount_minor === 0) return <span>Расчёты закрыты</span>;
  const positive = balance.net_amount_minor > 0;
  const prefix = current
    ? positive
      ? 'Тебе должны'
      : 'Ты должен'
    : positive
      ? 'Должны участнику'
      : 'Участник должен';
  return (
    <span className={positive ? 'balance-positive' : 'balance-negative'}>
      {prefix}{' '}
      <Money amountMinor={Math.abs(balance.net_amount_minor)} currency={balance.currency} />
    </span>
  );
}

export function BalancePage() {
  const { groupId } = useParams();
  const numericGroupId = Number(groupId);
  const { client, user } = useSession();
  const [data, setData] = useState<BalanceData>();
  const [error, setError] = useState<string>();

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!Number.isSafeInteger(numericGroupId) || numericGroupId <= 0) {
        setError('Некорректный идентификатор группы');
        return;
      }
      setError(undefined);
      try {
        const [group, members, balances] = await Promise.all([
          client.getGroup(numericGroupId, signal),
          client.listGroupMembers(numericGroupId, signal),
          client.getBalances(numericGroupId, signal),
        ]);
        setData({ balances, group, members });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(userErrorMessage(cause, 'Не удалось загрузить баланс'));
      }
    },
    [client, numericGroupId],
  );

  useEffect(() => {
    const controller = new AbortController();
    setData(undefined);
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  if (error && !data) return <ErrorState description={error} onRetry={() => void load()} />;
  if (!data) return <SkeletonList count={5} />;

  const balancesByUser = new Map<number, Balance[]>();
  data.balances.forEach((balance) => {
    const current = balancesByUser.get(balance.user_id) ?? [];
    current.push(balance);
    balancesByUser.set(balance.user_id, current);
  });
  const members = [...data.members].sort((left, right) => {
    if (left.user_id === user?.id) return -1;
    if (right.user_id === user?.id) return 1;
    return userName(left).localeCompare(userName(right), 'ru');
  });

  return (
    <div className="screen balance-page">
      <PageHeader
        action={
          <Button asChild size="xsmall" variant="ghost">
            <Link to={routes.settlements(String(data.group.id))}>Погашения</Link>
          </Button>
        }
        subtitle={data.group.name}
        title="Баланс"
      />
      <Container>
        <Flex direction="column" gap={16}>
          <Typography.Body color="secondary" variant="small">
            Нажмите на участника, чтобы увидеть, из каких операций сложился итог.
          </Typography.Body>
          {members.length ? (
            <CellList className="balance-page__list">
              {members.map((member) => {
                const memberBalances = balancesByUser.get(member.user_id) ?? [];
                const isCurrent = member.user_id === user?.id;
                const subtitle = memberBalances.length ? (
                  <Flex direction="column" gap={2}>
                    {memberBalances.map((balance) => (
                      <BalancePhrase
                        balance={balance}
                        current={isCurrent}
                        key={balance.currency}
                      />
                    ))}
                  </Flex>
                ) : (
                  'Расчёты закрыты'
                );
                return (
                  <CellSimple
                    asChild
                    before={<UserAvatar user={userFor(member)} />}
                    key={member.user_id}
                    showChevron
                    subtitle={subtitle}
                    title={isCurrent ? `${userName(member)} · Вы` : userName(member)}
                  >
                    <Link
                      aria-label={`Открыть расчёт пользователя ${userName(member)}`}
                      to={`${routes.balance(String(data.group.id))}?user=${member.user_id}`}
                    />
                  </CellSimple>
                );
              })}
            </CellList>
          ) : (
            <EmptyState description="Добавьте участников, чтобы вести расчёты." title="Нет участников" />
          )}
        </Flex>
      </Container>
    </div>
  );
}
