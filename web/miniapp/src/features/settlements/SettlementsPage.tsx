import { Button, CellList, CellSimple, Container, Flex, Input, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';

import type { Group, GroupMember, Settlement, SettlementPlanTransfer, User } from '../../api';
import { userErrorMessage } from '../../api';
import { FormField, FormMessage, useDirtyForm, useFormSubmit } from '../../components/form';
import {
  ConfirmDialog,
  EmptyState,
  ErrorState,
  Money,
  PageHeader,
  SkeletonList,
  StatusBadge,
} from '../../components/ui';
import { moneyInputFromMinor, parseMoneyInput } from '../../domain/money';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

interface SettlementPlanData {
  group: Group;
  members: GroupMember[];
  plan: SettlementPlanTransfer[];
  settlements: Settlement[];
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

const formatDate = (value: string) =>
  new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(value),
  );

interface SettlementFormProps {
  groupId: number;
  initialAmount?: number;
  initialCurrency?: string;
  initialReceiverId?: number;
  members: GroupMember[];
  onCreated(settlement: Settlement): void;
  senderId: number;
}

function SettlementForm({
  groupId,
  initialAmount,
  initialCurrency = 'RUB',
  initialReceiverId,
  members,
  onCreated,
  senderId,
}: SettlementFormProps) {
  const { client } = useSession();
  const receivers = members.filter((member) => member.user_id !== senderId);
  const fallbackReceiver = receivers[0]?.user_id ?? 0;
  const [receiverId, setReceiverId] = useState(
    receivers.some((member) => member.user_id === initialReceiverId)
      ? initialReceiverId!
      : fallbackReceiver,
  );
  const [currency, setCurrency] = useState(initialCurrency);
  const [amount, setAmount] = useState(() =>
    initialAmount ? moneyInputFromMinor(initialAmount, initialCurrency) : '',
  );
  const [dirty, setDirty] = useState(false);
  const amountMinor = parseMoneyInput(amount, currency);
  const amountError = amountMinor === undefined || amountMinor <= 0 ? 'Укажите положительную сумму' : undefined;
  const currencyError = /^[A-Z]{3}$/.test(currency) ? undefined : 'Введите код из трёх букв';
  const isValid = !amountError && !currencyError && receiverId > 0;

  useDirtyForm(dirty);

  const create = useCallback(
    () =>
      client.createSettlement(groupId, {
        amount_minor: amountMinor ?? 0,
        currency,
        receiver_user_id: receiverId,
        sender_user_id: senderId,
      }),
    [amountMinor, client, currency, groupId, receiverId, senderId],
  );
  const submit = useFormSubmit({
    isValid,
    onSubmit: create,
    onSuccess: (settlement: Settlement) => {
      setDirty(false);
      onCreated(settlement);
    },
    successMessage: 'Погашение отмечено и ждёт подтверждения',
  });

  if (!receivers.length) return null;

  return (
    <section aria-labelledby="settlement-form-title" className="settlement-form">
      <Typography.Headline asChild variant="small">
        <h3 id="settlement-form-title">Отметить погашение</h3>
      </Typography.Headline>
      <form className="form-stack" onSubmit={submit.handleSubmit}>
        <FormField htmlFor="settlement-receiver" label="Получатель" required>
          <select
            className="native-select"
            id="settlement-receiver"
            onChange={(event) => {
              setReceiverId(Number(event.target.value));
              setDirty(true);
            }}
            value={receiverId}
          >
            {receivers.map((member) => (
              <option key={member.user_id} value={member.user_id}>
                {userName(member)}
              </option>
            ))}
          </select>
        </FormField>
        <div className="expense-form__money-row">
          <FormField error={amount ? amountError : undefined} htmlFor="settlement-amount" label="Сумма" required>
            <Input
              aria-invalid={Boolean(amount && amountError)}
              id="settlement-amount"
              inputMode="decimal"
              onChange={(event) => {
                setAmount(event.target.value);
                setDirty(true);
              }}
              placeholder="0,00"
              value={amount}
            />
          </FormField>
          <FormField error={currencyError} htmlFor="settlement-currency" label="Валюта" required>
            <Input
              aria-invalid={Boolean(currencyError)}
              id="settlement-currency"
              maxLength={3}
              onChange={(event) => {
                setCurrency(event.target.value.toLocaleUpperCase('en-US'));
                setDirty(true);
              }}
              value={currency}
            />
          </FormField>
        </div>
        <FormMessage>{submit.error}</FormMessage>
        <FormMessage tone="success">{submit.feedback}</FormMessage>
        <Button disabled={!submit.canSubmit} loading={submit.submitting} size="medium" type="submit">
          Отметить погашение
        </Button>
      </form>
    </section>
  );
}

export function SettlementsPage() {
  const { groupId } = useParams();
  const numericGroupId = Number(groupId);
  const { client, user } = useSession();
  const [search] = useSearchParams();
  const [data, setData] = useState<SettlementPlanData>();
  const [error, setError] = useState<string>();
  const [actionError, setActionError] = useState<string>();
  const [createdSettlement, setCreatedSettlement] = useState<number>();
  const [confirming, setConfirming] = useState<Settlement>();
  const [confirmingLoading, setConfirmingLoading] = useState(false);
  const confirmationInFlight = useRef(false);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!Number.isSafeInteger(numericGroupId) || numericGroupId <= 0) {
        setError('Некорректный идентификатор группы');
        return;
      }
      setError(undefined);
      try {
        const [group, members, plan, settlementPage] = await Promise.all([
          client.getGroup(numericGroupId, signal),
          client.listGroupMembers(numericGroupId, signal),
          client.getSettlementPlan(numericGroupId, signal),
          client.listSettlements(numericGroupId, { limit: 50 }, signal),
        ]);
        setData({ group, members, plan, settlements: settlementPage.settlements });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(userErrorMessage(cause, 'Не удалось загрузить план погашений'));
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

  const memberById = new Map(data.members.map((member) => [member.user_id, member]));
  const querySenderId = Number(search.get('from'));
  const queryReceiverId = Number(search.get('to'));
  const queryAmount = Number(search.get('amount'));
  const queryCurrency = search.get('currency') ?? undefined;
  const hasValidPrefill =
    querySenderId === user?.id &&
    Number.isSafeInteger(queryReceiverId) &&
    queryReceiverId > 0 &&
    Number.isSafeInteger(queryAmount) &&
    queryAmount > 0;

  const confirmSettlement = async () => {
    if (!confirming || confirmationInFlight.current) return;
    confirmationInFlight.current = true;
    setConfirmingLoading(true);
    setActionError(undefined);
    try {
      await client.confirmSettlement(confirming.id);
      setConfirming(undefined);
      await load();
    } catch (cause) {
      setActionError(userErrorMessage(cause, 'Не удалось подтвердить погашение'));
    } finally {
      confirmationInFlight.current = false;
      setConfirmingLoading(false);
    }
  };

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

          {data.group.status === 'active' && user ? (
            <SettlementForm
              groupId={data.group.id}
              initialAmount={hasValidPrefill ? queryAmount : undefined}
              initialCurrency={hasValidPrefill ? queryCurrency : undefined}
              initialReceiverId={hasValidPrefill ? queryReceiverId : undefined}
              key={search.toString()}
              members={data.members}
              onCreated={(settlement) => {
                setCreatedSettlement(settlement.id);
                void load();
              }}
              senderId={user.id}
            />
          ) : null}

          <section aria-labelledby="settlements-history-title">
            <Typography.Headline asChild variant="small">
              <h3 id="settlements-history-title">История погашений</h3>
            </Typography.Headline>
            {createdSettlement ? (
              <FormMessage tone="success">
                Погашение #{createdSettlement} создано и ждёт подтверждения получателя.
              </FormMessage>
            ) : null}
            <FormMessage>{actionError}</FormMessage>
            {data.settlements.length ? (
              <CellList className="settlements-history">
                {data.settlements.map((settlement) => {
                  const pending = settlement.status === 'pending';
                  const canConfirm = pending && settlement.receiver_user_id === user?.id;
                  return (
                    <CellSimple
                      after={
                        canConfirm ? (
                          <Button
                            disabled={confirmingLoading}
                            onClick={() => setConfirming(settlement)}
                            size="xsmall"
                          >
                            Подтвердить получение
                          </Button>
                        ) : (
                          <StatusBadge tone={pending ? 'warning' : 'positive'}>
                            {pending ? 'Ожидает подтверждения' : 'Подтверждено'}
                          </StatusBadge>
                        )
                      }
                      key={settlement.id}
                      subtitle={`${formatDate(settlement.created_at)} · #${settlement.id}`}
                      title={
                        <span>
                          {userName(memberById.get(settlement.sender_user_id))} →{' '}
                          {userName(memberById.get(settlement.receiver_user_id))} ·{' '}
                          <Money
                            amountMinor={settlement.amount_minor}
                            currency={settlement.currency}
                          />
                        </span>
                      }
                    />
                  );
                })}
              </CellList>
            ) : (
              <Typography.Body color="secondary">Погашений пока нет.</Typography.Body>
            )}
          </section>
        </Flex>
      </Container>
      <ConfirmDialog
        confirmLabel="Подтвердить"
        description="Подтвердите, что деньги действительно получены. После этого баланс группы обновится."
        onCancel={() => setConfirming(undefined)}
        onConfirm={() => void confirmSettlement()}
        open={Boolean(confirming)}
        title="Деньги получены?"
      />
    </div>
  );
}
