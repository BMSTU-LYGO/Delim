import { Button, CellList, CellSimple, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';
import { Link, useLocation, useParams } from 'react-router-dom';

import type { Balance, Expense, Group, GroupBudgetSummary, GroupMember, MemberRole } from '../../api';
import { userErrorMessage } from '../../api';
import { Money, ErrorState, PageHeader, SkeletonList, StatusBadge } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';
import { ExportPanel } from './ExportPanel';
import { MaxChatPanel } from './MaxChatPanel';
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
  budget: GroupBudgetSummary;
  expenses: Expense[];
  group: Group;
  members: GroupMember[];
}

const memberName = (member?: GroupMember) => {
  if (!member?.user) return 'Участник';
  const name = [member.user.first_name, member.user.last_name].filter(Boolean).join(' ');
  return name || member.user.username || 'Участник';
};

const quickExpenses = [
  { description: 'Билеты', hint: 'кино, концерт или музей' },
  { description: 'Еда и напитки', hint: 'кафе, продукты или доставка' },
  { description: 'Транспорт', hint: 'такси, бензин или электричка' },
];

const activityLabels = {
  trip: 'Поездка',
  hike: 'Поход',
  event: 'Событие',
} as const;

const formatPlanDate = (date: string) =>
  new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short' }).format(new Date(`${date}T00:00:00`));

export function GroupDashboardPage() {
  const { groupId } = useParams();
  const location = useLocation();
  const { client, user } = useSession();
  const [data, setData] = useState<DashboardData>();
  const [error, setError] = useState<string>();
  const [planHelpOpen, setPlanHelpOpen] = useState(false);
  const numericGroupId = Number(groupId);
  const navigationState = location.state as { created?: boolean; joined?: boolean } | null;
  const created = Boolean(navigationState?.created);
  const joined = Boolean(navigationState?.joined);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!Number.isSafeInteger(numericGroupId) || numericGroupId <= 0) {
        setError('Некорректный идентификатор группы');
        return;
      }
      setError(undefined);
      try {
        const [group, expensePage, balances, members, budget] = await Promise.all([
          client.getGroup(numericGroupId, signal),
          client.listExpenses(numericGroupId, { limit: 5 }, signal),
          client.getBalances(numericGroupId, signal),
          client.listGroupMembers(numericGroupId, signal),
          client.getGroupBudgetSummary(numericGroupId, signal),
        ]);
        setData({ balances, budget, expenses: expensePage.expenses, group, members });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(userErrorMessage(cause, 'Не удалось загрузить группу'));
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

  const { balances, budget, expenses, group, members } = data;
  const currentBalances = balances.filter((balance) => balance.user_id === user?.id);
  const memberById = new Map(members.map((member) => [member.user_id, member]));
  const archived = group.status === 'archived';
  const canInvite = !archived && group.current_user_role !== 'member';
  const needsPeople = members.length < 2;
  const budgetPercent = budget.planned_budget_minor > 0
    ? Math.min(100, Math.round((budget.total_spend_minor / budget.planned_budget_minor) * 100))
    : 0;
  const budgetRemainingMinor = budget.planned_budget_minor - budget.total_spend_minor;
  const activityLabel = group.activity_type ? activityLabels[group.activity_type] : undefined;
  const dateLabel = group.start_date && group.end_date
    ? group.start_date === group.end_date
      ? formatPlanDate(group.start_date)
      : `${formatPlanDate(group.start_date)} — ${formatPlanDate(group.end_date)}`
    : undefined;
  const activityDetails = [activityLabel, group.location || undefined, dateLabel].filter(Boolean);

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
        <Flex direction="column" gap={16}>
          {activityDetails.length ? (
            <section aria-label="Детали плана" className="dashboard-card plan-context">
              {activityDetails.map((detail) => (
                <Typography.Body key={detail} variant="small">{detail}</Typography.Body>
              ))}
            </section>
          ) : null}
          {created && !archived ? (
            <div className="dashboard-notice" role="status">
              <Flex align="center" gap={12} justify="space-between">
                <Typography.Body>План создан — позовите друзей, чтобы делить траты.</Typography.Body>
                <Button asChild size="xsmall">
                  <Link to={routes.members(String(group.id))}>Пригласить</Link>
                </Button>
              </Flex>
            </div>
          ) : null}

          {joined ? (
            <div className="dashboard-notice" role="status">
              <Typography.Body>Вы присоединились к группе «{group.name}».</Typography.Body>
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

          {budget.planned_budget_minor > 0 ? (
            <section aria-labelledby="plan-budget" className="dashboard-card plan-budget">
              <Flex align="center" justify="space-between">
                <Typography.Body color="secondary" id="plan-budget" variant="medium">Бюджет плана</Typography.Body>
                <Typography.Body variant="medium"><Money amountMinor={budget.planned_budget_minor} /></Typography.Body>
              </Flex>
              <div aria-label={`Учтено ${budgetPercent}% бюджета с ожидающими тратами`} className="plan-budget__track" role="progressbar" aria-valuemax={100} aria-valuemin={0} aria-valuenow={budgetPercent}>
                <span style={{ width: `${budgetPercent}%` }} />
              </div>
              <div className="plan-budget__details">
                <Typography.Body color="secondary" variant="small">Подтверждено <Money amountMinor={budget.confirmed_spend_minor} /></Typography.Body>
                {budget.pending_spend_minor > 0 ? (
                  <Typography.Body color="secondary" variant="small">На проверке <Money amountMinor={budget.pending_spend_minor} /></Typography.Body>
                ) : null}
                <Typography.Body color={budgetRemainingMinor < 0 ? 'negative' : 'secondary'} variant="small">
                  {budgetRemainingMinor < 0 ? 'Превышение ' : 'Остаток '}
                  <Money amountMinor={Math.abs(budgetRemainingMinor)} />
                </Typography.Body>
              </div>
            </section>
          ) : null}

          {!archived ? (
            <div className="dashboard-actions">
              <Button asChild size="medium">
                <Link to={routes.newExpense(String(group.id))}>Добавить трату</Link>
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

          {!archived && expenses.length === 0 ? (
            <section aria-labelledby="quick-expenses" className="quick-expenses">
              <Typography.Headline asChild variant="small">
                <h3 id="quick-expenses">Что оплатили?</h3>
              </Typography.Headline>
              <Typography.Body color="secondary" variant="small">
                Выберите тип траты — описание подставится в форму.
              </Typography.Body>
              <div className="quick-expenses__grid">
                {quickExpenses.map((expense) => (
                  <Button asChild key={expense.description} size="small" variant="secondary">
                    <Link state={{ prefill: { description: expense.description } }} to={routes.newExpense(String(group.id))}>
                      <span>{expense.description}</span>
                      <small>{expense.hint}</small>
                    </Link>
                  </Button>
                ))}
              </div>
            </section>
          ) : null}

          {!archived ? <ReceiptUploadPanel groupId={group.id} /> : null}

          {!archived ? <MaxChatPanel /> : null}

          <nav aria-label="Разделы группы" className="dashboard-links">
            <Link className="dashboard-links__item" to={routes.balance(String(group.id))}>Баланс</Link>
            <Link className="dashboard-links__item" to={routes.settlements(String(group.id))}>Кому вернуть</Link>
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
              <Typography.Body color="secondary">Пока нет трат. Начните с билетов, еды или дороги.</Typography.Body>
            )}
          </section>

          <ExportPanel groupId={group.id} />

          {!archived ? (
            <section aria-labelledby="outing-guide" className="outing-guide">
              <Flex align="center" justify="space-between">
                <Typography.Headline asChild variant="small">
                  <h3 id="outing-guide">Как вести общий план</h3>
                </Typography.Headline>
                <Button
                  aria-expanded={planHelpOpen}
                  onClick={() => setPlanHelpOpen((open) => !open)}
                  size="xsmall"
                  type="button"
                  variant="ghost"
                >
                  {planHelpOpen ? 'Скрыть' : 'Подробнее'}
                </Button>
              </Flex>
              {planHelpOpen ? (
                <>
                  <Typography.Body color="secondary" variant="small">
                    {needsPeople
                      ? 'Сначала добавьте друзей, затем отмечайте, кто оплатил билеты, еду и дорогу.'
                      : 'Добавляйте траты по ходу встречи — баланс обновится для каждого участника.'}
                  </Typography.Body>
                  <ol className="outing-guide__steps">
                    <li className={needsPeople ? 'outing-guide__step outing-guide__step--active' : 'outing-guide__step'}>
                      {canInvite ? <Link to={routes.members(String(group.id))}>Пригласить друзей</Link> : 'Собрать участников'}
                    </li>
                    <li className={expenses.length ? 'outing-guide__step outing-guide__step--complete' : 'outing-guide__step'}>
                      Добавить первую трату
                    </li>
                    <li className="outing-guide__step">Закрыть долги после встречи</li>
                  </ol>
                </>
              ) : null}
            </section>
          ) : null}

        </Flex>
      </Container>
    </div>
  );
}
