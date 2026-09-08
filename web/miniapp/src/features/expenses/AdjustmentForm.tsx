import { Button, Flex, Input, Typography } from '@maxhub/max-ui';
import { useCallback, useMemo, useState } from 'react';

import type {
  Adjustment,
  AdjustmentType,
  Expense,
  GroupMember,
  User,
} from '../../api';
import { FormField, FormMessage, useDirtyForm, useFormSubmit } from '../../components/form';
import { Money } from '../../components/ui';
import { moneyInputFromMinor, parseMoneyInput } from '../../domain/money';
import { useSession } from '../../session/SessionProvider';

interface AdjustmentFormProps {
  adjustments: Adjustment[];
  expense: Expense;
  members: GroupMember[];
  onCreated(adjustment: Adjustment): void;
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

export function AdjustmentForm({
  adjustments,
  expense,
  members,
  onCreated,
}: AdjustmentFormProps) {
  const { client } = useSession();
  const participantIds = useMemo(() => {
    const allocated = [...new Set(expense.allocations.map((allocation) => allocation.user_id))];
    return allocated.length ? allocated : members.map((member) => member.user_id);
  }, [expense.allocations, members]);
  const memberById = useMemo(
    () => new Map(members.map((member) => [member.user_id, member])),
    [members],
  );
  const [adjustmentType, setAdjustmentType] = useState<AdjustmentType>('refund');
  const [amount, setAmount] = useState('');
  const [allocations, setAllocations] = useState<Record<number, string>>(() =>
    Object.fromEntries(participantIds.map((userId) => [userId, ''])),
  );
  const [dirty, setDirty] = useState(false);
  const amountMinor = parseMoneyInput(amount, expense.currency);
  const alreadyRefunded = adjustments
    .filter((adjustment) => adjustment.type === 'refund')
    .reduce((sum, adjustment) => sum + adjustment.amount_minor, 0);
  const refundAvailable = Math.max(0, expense.amount_minor - alreadyRefunded);
  const amountError =
    amountMinor === undefined || amountMinor <= 0
      ? 'Укажите положительную сумму'
      : adjustmentType === 'refund' && amountMinor > refundAvailable
        ? 'Сумма возврата превышает остаток расхода'
        : undefined;
  const parsedAllocations = useMemo(
    () =>
      participantIds.map((userId) => ({
        amount_minor: parseMoneyInput(allocations[userId] ?? '', expense.currency),
        user_id: userId,
      })),
    [allocations, expense.currency, participantIds],
  );
  const allocationTotal = parsedAllocations.reduce(
    (sum, allocation) => sum + (allocation.amount_minor ?? 0),
    0,
  );
  const allocationError = parsedAllocations.some(
    (allocation) => allocation.amount_minor === undefined,
  )
    ? 'Укажите сумму для каждого участника'
    : allocationTotal !== amountMinor
      ? 'Сумма распределения должна совпадать с суммой операции'
      : undefined;
  const isValid = !amountError && !allocationError && participantIds.length > 0;

  useDirtyForm(dirty);

  const splitEqually = () => {
    if (!amountMinor || amountMinor <= 0 || !participantIds.length) return;
    const sortedIds = [...participantIds].sort((left, right) => left - right);
    const base = Math.floor(amountMinor / sortedIds.length);
    const remainder = amountMinor % sortedIds.length;
    setAllocations(
      Object.fromEntries(
        sortedIds.map((userId, index) => [
          userId,
          moneyInputFromMinor(base + (index < remainder ? 1 : 0), expense.currency),
        ]),
      ),
    );
    setDirty(true);
  };

  const create = useCallback(
    () =>
      client.createAdjustment(expense.id, {
        allocations: parsedAllocations.map((allocation) => ({
          amount_minor: allocation.amount_minor ?? 0,
          user_id: allocation.user_id,
        })),
        amount_minor: amountMinor ?? 0,
        currency: expense.currency,
        type: adjustmentType,
      }),
    [adjustmentType, amountMinor, client, expense.currency, expense.id, parsedAllocations],
  );
  const submit = useFormSubmit({
    isValid,
    onSubmit: create,
    onSuccess: (adjustment: Adjustment) => {
      setDirty(false);
      onCreated(adjustment);
    },
    successMessage: 'Операция сохранена отдельно от исходного расхода',
  });

  return (
    <section aria-labelledby="adjustment-form-title" className="adjustment-form">
      <Typography.Headline asChild variant="small">
        <h3 id="adjustment-form-title">Возврат или корректировка</h3>
      </Typography.Headline>
      <div className="expense-notice">
        <Typography.Body color="secondary" variant="small">
          Исходный расход на{' '}
          <Money amountMinor={expense.amount_minor} currency={expense.currency} /> останется в
          истории без изменений.
        </Typography.Body>
      </div>
      <form className="form-stack" onSubmit={submit.handleSubmit}>
        <FormField htmlFor="adjustment-type" label="Тип операции" required>
          <select
            className="native-select"
            id="adjustment-type"
            onChange={(event) => {
              setAdjustmentType(event.target.value as AdjustmentType);
              setDirty(true);
            }}
            value={adjustmentType}
          >
            <option value="refund">Возврат</option>
            <option value="correction">Корректировка</option>
          </select>
        </FormField>
        <FormField
          error={amount ? amountError : undefined}
          hint={
            adjustmentType === 'refund'
              ? `Доступно к возврату: ${moneyInputFromMinor(refundAvailable, expense.currency)} ${expense.currency}`
              : 'Корректировка увеличит сумму обязательств'
          }
          htmlFor="adjustment-amount"
          label="Сумма"
          required
        >
          <Input
            aria-invalid={Boolean(amount && amountError)}
            id="adjustment-amount"
            inputMode="decimal"
            onChange={(event) => {
              setAmount(event.target.value);
              setDirty(true);
            }}
            placeholder="0,00"
            value={amount}
          />
        </FormField>

        <Flex align="center" gap={12} justify="space-between">
          <Typography.Body variant="large-strong">Распределение</Typography.Body>
          <Button
            disabled={!amountMinor || amountMinor <= 0}
            onClick={splitEqually}
            size="small"
            type="button"
            variant="ghost"
          >
            Поровну
          </Button>
        </Flex>
        <div className="adjustment-form__allocations">
          {participantIds.map((userId) => {
            const inputId = `adjustment-allocation-${userId}`;
            const value = allocations[userId] ?? '';
            return (
              <FormField htmlFor={inputId} key={userId} label={userName(memberById.get(userId))}>
                <Input
                  aria-invalid={Boolean(value && parseMoneyInput(value, expense.currency) === undefined)}
                  id={inputId}
                  inputMode="decimal"
                  onChange={(event) => {
                    setAllocations((current) => ({ ...current, [userId]: event.target.value }));
                    setDirty(true);
                  }}
                  placeholder="0,00"
                  value={value}
                />
              </FormField>
            );
          })}
        </div>
        {amount && allocationError ? <FormMessage>{allocationError}</FormMessage> : null}
        <FormMessage>{submit.error}</FormMessage>
        <FormMessage tone="success">{submit.feedback}</FormMessage>
        <Button disabled={!submit.canSubmit} loading={submit.submitting} size="medium" type="submit">
          Сохранить операцию
        </Button>
      </form>
    </section>
  );
}
