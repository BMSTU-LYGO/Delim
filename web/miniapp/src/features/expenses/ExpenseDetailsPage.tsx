import { Button, CellList, CellSimple, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';

import type {
  Adjustment,
  Expense,
  ExpenseStatus,
  Group,
  GroupMember,
  SplitType,
  User,
} from '../../api';
import { userErrorMessage } from '../../api';
import { FormMessage } from '../../components/form';
import {
  ConfirmDialog,
  ErrorState,
  Money,
  PageHeader,
  SkeletonList,
  StatusBadge,
  StickyActionBar,
} from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';
import { AdjustmentForm } from './AdjustmentForm';

const statusView: Record<ExpenseStatus, { label: string; tone: 'warning' | 'positive' | 'neutral' }> = {
  pending: { label: 'На проверке', tone: 'warning' },
  confirmed: { label: 'Подтверждён', tone: 'positive' },
  cancelled: { label: 'Отменён', tone: 'neutral' },
};

const splitLabels: Record<SplitType, string> = {
  equal: 'Поровну',
  shares: 'По долям',
  percentage: 'По процентам',
  fixed: 'Точные суммы',
  item: 'По позициям',
};

interface ExpenseDetailsData {
  adjustments: Adjustment[];
  expense: Expense;
  group: Group;
  members: GroupMember[];
}

const userFor = (member?: GroupMember): User =>
  member?.user ?? {
    first_name: '',
    id: member?.user_id ?? 0,
    last_name: '',
    max_user_id: 0,
    username: 'Участник',
  };

const userName = (member?: GroupMember) => {
  const user = userFor(member);
  return [user.first_name, user.last_name].filter(Boolean).join(' ') || user.username;
};

const formatDate = (value: string) =>
  new Intl.DateTimeFormat('ru-RU', { dateStyle: 'long', timeStyle: 'short' }).format(
    new Date(value),
  );

export function ExpenseDetailsPage() {
  const { expenseId } = useParams();
  const numericExpenseId = Number(expenseId);
  const location = useLocation();
  const navigate = useNavigate();
  const { client, user } = useSession();
  const [data, setData] = useState<ExpenseDetailsData>();
  const [error, setError] = useState<string>();
  const [actionError, setActionError] = useState<string>();
  const [action, setAction] = useState<'confirm' | 'cancel'>();
  const [actionLoading, setActionLoading] = useState(false);
  const actionInFlight = useRef(false);
  const created = Boolean((location.state as { created?: boolean } | null)?.created);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!Number.isSafeInteger(numericExpenseId) || numericExpenseId <= 0) {
        setError('Некорректный идентификатор расхода');
        return;
      }
      setError(undefined);
      try {
        const expense = await client.getExpense(numericExpenseId, signal);
        const [group, members, adjustments] = await Promise.all([
          client.getGroup(expense.group_id, signal),
          client.listGroupMembers(expense.group_id, signal),
          client.listAdjustments(numericExpenseId, signal),
        ]);
        setData({ adjustments, expense, group, members });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(userErrorMessage(cause, 'Не удалось загрузить расход'));
      }
    },
    [client, numericExpenseId],
  );

  useEffect(() => {
    const controller = new AbortController();
    setData(undefined);
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const runAction = async (nextAction: 'confirm' | 'cancel') => {
    if (actionInFlight.current) return;
    actionInFlight.current = true;
    setActionLoading(true);
    setActionError(undefined);
    try {
      const expense = nextAction === 'confirm'
        ? await client.confirmExpense(numericExpenseId)
        : await client.cancelExpense(numericExpenseId);
      setData((current) => current && { ...current, expense });
    } catch (cause) {
      setActionError(userErrorMessage(cause, 'Не удалось изменить статус расхода'));
    } finally {
      actionInFlight.current = false;
      setActionLoading(false);
      setAction(undefined);
    }
  };

  if (error && !data) return <ErrorState description={error} onRetry={() => void load()} />;
  if (!data) return <SkeletonList count={6} />;

  const { adjustments, expense, group, members } = data;
  const memberById = new Map(members.map((member) => [member.user_id, member]));
  const status = statusView[expense.status];
  const pending = expense.status === 'pending';
  const mutable = pending && group.status === 'active';
  const adjustmentOpen = new URLSearchParams(location.search).get('adjustment') === '1';
  const canAdjust =
    expense.status === 'confirmed' &&
    group.status === 'active' &&
    (expense.created_by === user?.id || group.current_user_role !== 'member');

  return (
    <div className="screen expense-details">
      <PageHeader
        action={<StatusBadge tone={status.tone}>{status.label}</StatusBadge>}
        subtitle={group.name}
        title={expense.description || 'Расход без названия'}
      />
      <Container className="expense-details__content">
        <Flex direction="column" gap={20}>
          {created ? (
            <div className="expense-notice" role="status">
              Расход сохранён и ожидает подтверждения.
            </div>
          ) : null}
          <section className="expense-summary">
            <Typography.Body color="secondary" variant="medium">
              Сумма
            </Typography.Body>
            <Typography.Display asChild>
              <Money amountMinor={expense.amount_minor} currency={expense.currency} />
            </Typography.Display>
            <dl className="expense-metadata">
              <div>
                <dt>Заплатил</dt>
                <dd>{userName(memberById.get(expense.payer_user_id))}</dd>
              </div>
              <div>
                <dt>Дата</dt>
                <dd>{formatDate(expense.expense_date)}</dd>
              </div>
              <div>
                <dt>Разделение</dt>
                <dd>{splitLabels[expense.split_type]}</dd>
              </div>
            </dl>
          </section>

          <section aria-labelledby="allocations-title">
            <Typography.Headline asChild variant="small">
              <h3 id="allocations-title">Кто сколько должен</h3>
            </Typography.Headline>
            <CellList className="expense-details__list">
              {expense.allocations.map((allocation) => (
                <CellSimple
                  after={<Money amountMinor={allocation.amount_minor} currency={expense.currency} />}
                  key={allocation.id}
                  subtitle={allocation.expense_item_id ? 'По позиции чека' : undefined}
                  title={userName(memberById.get(allocation.user_id))}
                />
              ))}
            </CellList>
          </section>

          {expense.items.length ? (
            <section aria-labelledby="expense-items-title">
              <Typography.Headline asChild variant="small">
                <h3 id="expense-items-title">Позиции</h3>
              </Typography.Headline>
              <CellList className="expense-details__list">
                {expense.items.map((item) => (
                  <CellSimple
                    after={<Money amountMinor={item.amount_minor} currency={expense.currency} />}
                    key={item.id}
                    overline={`Позиция ${item.position + 1}`}
                    title={item.name}
                  />
                ))}
              </CellList>
            </section>
          ) : null}

          {adjustments.length ? (
            <section aria-labelledby="adjustments-title">
              <Typography.Headline asChild variant="small">
                <h3 id="adjustments-title">Возвраты и корректировки</h3>
              </Typography.Headline>
              <div className="adjustment-history">
                {adjustments.map((adjustment) => (
                  <article className="adjustment-history__item" key={adjustment.id}>
                    <Flex align="center" gap={12} justify="space-between">
                      <div>
                        <Typography.Body asChild variant="large-strong">
                          <h4>{adjustment.type === 'refund' ? 'Возврат' : 'Корректировка'}</h4>
                        </Typography.Body>
                        <Typography.Body color="secondary" variant="small">
                          {formatDate(adjustment.created_at)} · участников:{' '}
                          {adjustment.allocations.length}
                        </Typography.Body>
                      </div>
                      <Typography.Body variant="large-strong">
                        {adjustment.type === 'refund' ? '−' : '+'}
                        <Money
                          amountMinor={adjustment.amount_minor}
                          currency={adjustment.currency}
                        />
                      </Typography.Body>
                    </Flex>
                  </article>
                ))}
              </div>
            </section>
          ) : null}

          {pending && !mutable ? (
            <div className="expense-notice">Группа в архиве: расход доступен только для чтения.</div>
          ) : null}
          {canAdjust && !adjustmentOpen ? (
            <Button asChild size="medium" variant="secondary">
              <Link to={`${routes.expense(String(expense.id))}?adjustment=1`}>
                Возврат или корректировка
              </Link>
            </Button>
          ) : null}
          {canAdjust && adjustmentOpen ? (
            <AdjustmentForm
              adjustments={adjustments}
              expense={expense}
              members={members}
              onCancel={() => navigate(routes.expense(String(expense.id)), { replace: true })}
              onCreated={(adjustment) => {
                setData((current) =>
                  current
                    ? { ...current, adjustments: [...current.adjustments, adjustment] }
                    : current,
                );
                navigate(routes.expense(String(expense.id)), {
                  replace: true,
                  state: { adjusted: true },
                });
              }}
            />
          ) : null}
          <FormMessage>{actionError}</FormMessage>
        </Flex>
      </Container>

      {mutable ? (
        <StickyActionBar>
          <Button asChild size="medium" variant="secondary">
            <Link to={routes.editExpense(String(expense.id))}>Изменить</Link>
          </Button>
          <Button
            disabled={actionLoading}
            onClick={() => setAction('cancel')}
            size="medium"
            variant="ghost"
          >
            Отменить
          </Button>
          <Button disabled={actionLoading} onClick={() => setAction('confirm')} size="medium">
            Подтвердить
          </Button>
        </StickyActionBar>
      ) : null}

      <ConfirmDialog
        confirmLabel="Подтвердить"
        description="После подтверждения расход нельзя будет редактировать. Исправления оформляются отдельным возвратом."
        onCancel={() => setAction(undefined)}
        onConfirm={() => void runAction('confirm')}
        open={action === 'confirm'}
        title="Подтвердить расход?"
      />
      <ConfirmDialog
        confirmLabel="Отменить расход"
        description="Отменённый расход останется в истории, но не будет влиять на баланс."
        destructive
        onCancel={() => setAction(undefined)}
        onConfirm={() => void runAction('cancel')}
        open={action === 'cancel'}
        title="Отменить расход?"
      />
    </div>
  );
}
