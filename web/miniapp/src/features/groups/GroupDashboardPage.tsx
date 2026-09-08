import { Button, CellList, CellSimple, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';
import { Link, useLocation, useParams } from 'react-router-dom';

import type { Balance, Expense, Group, GroupMember, MemberRole } from '../../api';
import { Money, ErrorState, PageHeader, SkeletonList, StatusBadge } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';
import { InvitePanel } from './InvitePanel';
import { ArchiveGroupAction } from './ArchiveGroupAction';
import { ExportPanel } from './ExportPanel';
import { ReceiptUploadPanel } from '../receipts/ReceiptUploadPanel';

const roleLabels: Record<MemberRole, string> = {
  owner: 'Владелец',
  admin: 'Администратор',
  member: 'Участник',
};

const expenseStatus = {
  pending: ['На проверке', 'warning'],
  confirmed: ['Подтверждён', 'positive'],
  cancelled: ['Отменён', 'neutral'],
} as const;

interface DashboardData {
  balances: Balance[];
  expenses: Expense[];
  group: Group;
  members: GroupMember[];
}

const memberName = (member?: GroupMember) => {
  if (!member?.user) return 'Участник';
  const name = [member.user.first_name, member.user.last_name].filter(Boolean).join(' ');
  return name || member.user.username || 'Участник';
};

export function GroupDashboardPage() {
  const { groupId } = useParams();
  const location = useLocation();
  const { client, user } = useSession();
  const [data, setData] = useState<DashboardData>();
  const [error, setError] = useState<string>();
  const numericGroupId = Number(groupId);
  const created = Boolean((location.state as { created?: boolean } | null)?.created);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!Number.isSafeInteger(numericGroupId) || numericGroupId <= 0) {
        setError('Некорректный идентификатор группы');
        return;
      }
      setError(undefined);
      try {
        const [group, expensePage, balances, members] = await Promise.all([
          client.getGroup(numericGroupId, signal),
          client.listExpenses(numericGroupId, { limit: 5 }, signal),
          client.getBalances(numericGroupId, signal),
          client.listGroupMembers(numericGroupId, signal),
        ]);
        setData({ balances, expenses: expensePage.expenses, group, members });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(cause instanceof Error ? cause.message : 'Не удалось загрузить группу');
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

  const { balances, expenses, group, members } = data;
  const currentBalances = balances.filter((balance) => balance.user_id === user?.id);
  const memberById = new Map(members.map((member) => [member.user_id, member]));
  const archived = group.status === 'archived';

  return (
    <div className="screen group-dashboard">
      <PageHeader
        action={
          <Button asChild size="xsmall" variant="ghost">
            <Link aria-label="Настройки группы" to={routes.members(String(group.id))}>
              Настройки
            </Link>
          </Button>
        }
        subtitle={roleLabels[group.current_user_role]}
        title={group.name}
      />
      <Container>
        <Flex direction="column" gap={20}>
          {created && !archived ? (
            <div className="dashboard-notice" role="status">
              <Flex align="center" gap={12} justify="space-between">
                <Typography.Body>Группа создана — пригласите участников.</Typography.Body>
                <Button asChild size="xsmall">
                  <a href="#invite">Пригласить</a>
                </Button>
              </Flex>
            </div>
          ) : null}

          <section aria-labelledby="my-balance" className="dashboard-card dashboard-balance">
            <Typography.Body color="secondary" id="my-balance" variant="medium">
              Ваш баланс
            </Typography.Body>
            <Flex direction="column" gap={2}>
              {currentBalances.length ? (
                currentBalances.map((balance) => (
                  <Typography.Headline asChild key={balance.currency} variant="large-strong">
                    <Money
                      amountMinor={balance.net_amount_minor}
                      className={balance.net_amount_minor < 0 ? 'money-negative' : 'money-positive'}
                      currency={balance.currency}
                      sign="always"
                    />
                  </Typography.Headline>
                ))
              ) : (
                <Typography.Headline asChild variant="large-strong">
                  <Money amountMinor={0} />
                </Typography.Headline>
              )}
            </Flex>
            <Typography.Body color="tertiary" variant="small">
              Плюс — вам должны, минус — должны вы
            </Typography.Body>
          </section>

          {!archived ? (
            <div className="dashboard-actions">
              <Button asChild size="medium">
                <Link to={routes.newExpense(String(group.id))}>Добавить расход</Link>
              </Button>
              <Button asChild size="medium" variant="secondary">
                <Link to={`${routes.group(String(group.id))}?receipt=1#receipt-upload`}>Сканировать чек</Link>
              </Button>
            </div>
          ) : (
            <div className="dashboard-notice">
              <StatusBadge>Группа в архиве</StatusBadge>
              <Typography.Body color="secondary" variant="small">
                История доступна только для чтения.
              </Typography.Body>
            </div>
          )}

          {!archived && group.current_user_role !== 'member' ? (
            <InvitePanel groupId={group.id} />
          ) : null}

          {!archived ? <ReceiptUploadPanel groupId={group.id} /> : null}

          <nav aria-label="Разделы группы" className="dashboard-links">
            <Button asChild size="small" variant="secondary">
              <Link to={routes.balance(String(group.id))}>Баланс</Link>
            </Button>
            <Button asChild size="small" variant="secondary">
              <Link to={routes.members(String(group.id))}>Участники</Link>
            </Button>
            <Button asChild size="small" variant="secondary">
              <Link to={routes.settlements(String(group.id))}>Погашения</Link>
            </Button>
          </nav>

          <section aria-labelledby="recent-expenses">
            <Typography.Headline asChild variant="small">
              <h3 className="dashboard-section-title" id="recent-expenses">
                Последние расходы
              </h3>
            </Typography.Headline>
            {expenses.length ? (
              <CellList className="dashboard-expenses">
                {expenses.map((expense) => {
                  const [label, tone] = expenseStatus[expense.status];
                  return (
                    <CellSimple
                      after={<Money amountMinor={expense.amount_minor} currency={expense.currency} />}
                      asChild
                      key={expense.id}
                      overline={<StatusBadge tone={tone}>{label}</StatusBadge>}
                      showChevron
                      subtitle={`Оплатил: ${memberName(memberById.get(expense.payer_user_id))}`}
                      title={expense.description || 'Расход без названия'}
                    >
                      <Link to={routes.expense(String(expense.id))} />
                    </CellSimple>
                  );
                })}
              </CellList>
            ) : (
              <Typography.Body color="secondary">Расходов пока нет.</Typography.Body>
            )}
          </section>

          <ExportPanel groupId={group.id} />

          <ArchiveGroupAction
            group={group}
            onArchived={(archivedGroup) =>
              setData((current) => current && { ...current, group: archivedGroup })
            }
          />
        </Flex>
      </Container>
    </div>
  );
}
