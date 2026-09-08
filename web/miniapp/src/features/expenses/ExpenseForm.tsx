import { Button, Container, Flex, Input, Typography } from '@maxhub/max-ui';
import { useCallback, useMemo, useState } from 'react';

import type { Expense, ExpenseInput, Group, GroupMember, SplitType, User } from '../../api';
import { FormField, FormMessage, useDirtyForm, useFormSubmit } from '../../components/form';
import { PageHeader, StickyActionBar, UserAvatar } from '../../components/ui';
import { moneyInputFromMinor, parseMoneyInput } from '../../domain/money';
import {
  buildSplitParticipants,
  defaultSplitValues,
  SplitModeEditor,
  splitValidation,
  type SplitValues,
} from './SplitModeEditor';
import {
  buildExpenseItems,
  createDraftItem,
  ItemSplitEditor,
  itemSplitValidation,
  type ExpenseDraftItem,
} from './ItemSplitEditor';

const splitLabels: Record<SplitType, string> = {
  equal: 'Поровну',
  shares: 'По долям',
  percentage: 'По процентам',
  fixed: 'Точные суммы',
  item: 'По позициям',
};

const localDateTime = () => {
  const date = new Date(Date.now() - new Date().getTimezoneOffset() * 60_000);
  return date.toISOString().slice(0, 16);
};

const dateTimeInput = (value: string) => {
  const date = new Date(value);
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
};

const userFor = (member: GroupMember): User =>
  member.user ?? {
    first_name: '',
    id: member.user_id,
    last_name: '',
    max_user_id: 0,
    username: `id${member.user_id}`,
  };

const userName = (member: GroupMember) => {
  const user = userFor(member);
  return [user.first_name, user.last_name].filter(Boolean).join(' ') || user.username;
};

interface ExpenseFormProps {
  group: Group;
  initialExpense?: Expense;
  members: GroupMember[];
  onConflict?(error: unknown): void;
  onSave(input: ExpenseInput): Promise<Expense>;
  onSaved(expense: Expense): void;
  submitLabel?: string;
  title?: string;
}

const valuesFromExpense = (expense: Expense): SplitValues => {
  const amounts = new Map<number, number>();
  expense.allocations.forEach((allocation) =>
    amounts.set(allocation.user_id, (amounts.get(allocation.user_id) ?? 0) + allocation.amount_minor),
  );
  if (expense.split_type === 'fixed') {
    return Object.fromEntries(
      [...amounts].map(([userId, amount]) => [
        userId,
        moneyInputFromMinor(amount, expense.currency),
      ]),
    );
  }
  if (expense.split_type === 'shares') {
    return Object.fromEntries([...amounts].map(([userId, amount]) => [userId, String(amount)]));
  }
  if (expense.split_type === 'percentage') {
    const entries = [...amounts].sort(([left], [right]) => left - right);
    const total = entries.reduce((sum, [, amount]) => sum + BigInt(amount), 0n);
    let allocated = 0;
    const basisPoints = entries.map(([userId, amount]) => {
      const value = total ? Number((BigInt(amount) * 10_000n) / total) : 0;
      allocated += value;
      return [userId, value] as const;
    });
    for (let index = 0; index < 10_000 - allocated; index += 1) {
      if (basisPoints[index]) basisPoints[index] = [basisPoints[index][0], basisPoints[index][1] + 1];
    }
    return Object.fromEntries(
      basisPoints.map(([userId, value]) => {
        const major = Math.trunc(value / 100);
        const fraction = String(value % 100).padStart(2, '0').replace(/0+$/, '');
        return [userId, fraction ? `${major},${fraction}` : String(major)];
      }),
    );
  }
  return {};
};

const itemsFromExpense = (expense: Expense): ExpenseDraftItem[] =>
  expense.items.map((item) => ({
    ...createDraftItem(item.amount_minor, expense.currency),
    name: item.name,
    participantIds: [
      ...new Set(
        expense.allocations
          .filter((allocation) => allocation.expense_item_id === item.id)
          .map((allocation) => allocation.user_id),
      ),
    ],
  }));

export function ExpenseForm({
  group,
  initialExpense,
  members,
  onConflict,
  onSave,
  onSaved,
  submitLabel = 'Сохранить расход',
  title = 'Новый расход',
}: ExpenseFormProps) {
  const [initial] = useState(() => ({
    amount: initialExpense
      ? moneyInputFromMinor(initialExpense.amount_minor, initialExpense.currency)
      : '',
    currency: initialExpense?.currency ?? 'RUB',
    date: initialExpense ? dateTimeInput(initialExpense.expense_date) : localDateTime(),
    description: initialExpense?.description ?? '',
    items: initialExpense ? itemsFromExpense(initialExpense) : [],
    participantIds: initialExpense
      ? [...new Set(initialExpense.allocations.map((allocation) => allocation.user_id))]
      : members.map((member) => member.user_id),
    payerId: initialExpense?.payer_user_id ?? group.owner_id,
    splitType: initialExpense?.split_type ?? 'equal',
    splitValues: initialExpense ? valuesFromExpense(initialExpense) : {},
  }));
  const [description, setDescription] = useState(initial.description);
  const [amount, setAmount] = useState(initial.amount);
  const [currency, setCurrency] = useState(initial.currency);
  const [payerId, setPayerId] = useState(initial.payerId);
  const [expenseDate, setExpenseDate] = useState(initial.date);
  const [splitType, setSplitType] = useState<SplitType>(initial.splitType);
  const [splitValues, setSplitValues] = useState<SplitValues>(initial.splitValues);
  const [items, setItems] = useState<ExpenseDraftItem[]>(initial.items);
  const [participantIds, setParticipantIds] = useState<number[]>(initial.participantIds);
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [dirty, setDirty] = useState(false);
  const [committed, setCommitted] = useState(false);

  const amountMinor = useMemo(() => parseMoneyInput(amount, currency), [amount, currency]);
  const amountError = amountMinor === undefined
    ? 'Введите сумму в формате 6240,50'
    : amountMinor <= 0
      ? 'Сумма должна быть больше нуля'
      : undefined;
  const currencyError = /^[A-Z]{3}$/.test(currency) ? undefined : 'Введите код из трёх букв';
  const participantsError =
    splitType === 'item' || participantIds.length ? undefined : 'Выберите хотя бы одного участника';
  const splitError = splitType === 'item'
    ? itemSplitValidation(items, amountMinor, currency)
    : splitValidation(splitType, participantIds, splitValues, amountMinor, currency);
  const validDate = !Number.isNaN(new Date(expenseDate).getTime());
  const isValid =
    !amountError &&
    !currencyError &&
    !participantsError &&
    !splitError &&
    validDate &&
    payerId > 0;

  useDirtyForm(dirty && !committed);

  const buildInput = useCallback(
    (): ExpenseInput => ({
      amount_minor: amountMinor ?? 0,
      currency,
      description: description.trim(),
      expense_date: new Date(expenseDate).toISOString(),
      items: splitType === 'item' ? buildExpenseItems(items, currency) : [],
      participants:
        splitType === 'item' ? [] : buildSplitParticipants(splitType, participantIds, splitValues, currency),
      payer_user_id: payerId,
      split_type: splitType,
    }),
    [
      amountMinor,
      currency,
      description,
      expenseDate,
      items,
      participantIds,
      payerId,
      splitType,
      splitValues,
    ],
  );
  const submitExpense = useCallback(() => onSave(buildInput()), [buildInput, onSave]);
  const finish = useCallback(
    (expense: Expense) => {
      setCommitted(true);
      onSaved(expense);
    },
    [onSaved],
  );
  const submit = useFormSubmit({
    isValid,
    onError: onConflict,
    onSubmit: submitExpense,
    onSuccess: finish,
    successMessage: 'Расход сохранён',
  });

  const touch = (field: string) => setTouched((current) => ({ ...current, [field]: true }));
  const toggleParticipant = (userId: number) => {
    setDirty(true);
    touch('participants');
    setParticipantIds((current) => {
      const next = current.includes(userId)
        ? current.filter((id) => id !== userId)
        : [...current, userId];
      if (splitType !== 'equal' && splitType !== 'item') {
        setSplitValues(defaultSplitValues(splitType, next, amountMinor, currency));
      }
      return next;
    });
  };

  const changeSplitType = (next: SplitType) => {
    setDirty(true);
    setSplitType(next);
    setSplitValues(defaultSplitValues(next, participantIds, amountMinor, currency));
    if (next === 'item' && items.length === 0) {
      setItems([createDraftItem(amountMinor, currency, participantIds)]);
    }
  };

  return (
    <div className="screen expense-form-page">
      <PageHeader subtitle={group.name} title={title} />
      <Container className="expense-form-page__content">
        <form className="form-stack" id="expense-form" onSubmit={submit.handleSubmit}>
          <FormField htmlFor="expense-description" label="Описание">
            <Input
              autoComplete="off"
              id="expense-description"
              maxLength={240}
              onChange={(event) => {
                setDescription(event.target.value);
                setDirty(true);
              }}
              placeholder="Например, ужин"
              value={description}
            />
          </FormField>

          <div className="expense-form__money-row">
            <FormField
              error={touched.amount ? amountError : undefined}
              htmlFor="expense-amount"
              label="Сумма"
              required
            >
              <Input
                aria-describedby="expense-amount-message"
                aria-invalid={touched.amount && Boolean(amountError)}
                id="expense-amount"
                inputMode="decimal"
                onBlur={() => touch('amount')}
                onChange={(event) => {
                  setAmount(event.target.value);
                  setDirty(true);
                }}
                placeholder="0,00"
                value={amount}
              />
            </FormField>
            <FormField
              error={touched.currency ? currencyError : undefined}
              htmlFor="expense-currency"
              label="Валюта"
              required
            >
              <Input
                aria-describedby="expense-currency-message"
                aria-invalid={touched.currency && Boolean(currencyError)}
                id="expense-currency"
                maxLength={3}
                onBlur={() => touch('currency')}
                onChange={(event) => {
                  setCurrency(event.target.value.toLocaleUpperCase('en-US'));
                  setDirty(true);
                }}
                value={currency}
              />
            </FormField>
          </div>

          <FormField htmlFor="expense-payer" label="Кто заплатил" required>
            <select
              className="native-select"
              id="expense-payer"
              onChange={(event) => {
                setPayerId(Number(event.target.value));
                setDirty(true);
              }}
              value={payerId}
            >
              {members.map((member) => (
                <option key={member.user_id} value={member.user_id}>
                  {userName(member)}
                </option>
              ))}
            </select>
          </FormField>

          <FormField htmlFor="expense-date" label="Дата и время" required>
            <Input
              id="expense-date"
              onChange={(event) => {
                setExpenseDate(event.target.value);
                setDirty(true);
              }}
              type="datetime-local"
              value={expenseDate}
            />
          </FormField>

          {splitType !== 'item' ? (
            <FormField
              error={touched.participants ? participantsError : undefined}
              label="Участники расхода"
              required
            >
              <div className="participant-picker">
                {members.map((member) => {
                  const memberUser = userFor(member);
                  return (
                    <label className="participant-picker__item" key={member.user_id}>
                      <input
                        checked={participantIds.includes(member.user_id)}
                        onChange={() => toggleParticipant(member.user_id)}
                        type="checkbox"
                      />
                      <UserAvatar size={36} user={memberUser} />
                      <span>{userName(member)}</span>
                    </label>
                  );
                })}
              </div>
            </FormField>
          ) : null}

          <FormField error={splitError} htmlFor="expense-split" label="Как разделить" required>
            <select
              className="native-select"
              id="expense-split"
              onChange={(event) => changeSplitType(event.target.value as SplitType)}
              value={splitType}
            >
              {(Object.entries(splitLabels) as Array<[SplitType, string]>).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </FormField>

          <SplitModeEditor
            amountMinor={amountMinor}
            currency={currency}
            members={members}
            onChange={(userId, value) => {
              setSplitValues((current) => ({ ...current, [userId]: value }));
              setDirty(true);
            }}
            participantIds={participantIds}
            splitType={splitType}
            values={splitValues}
          />
          {splitType === 'item' ? (
            <ItemSplitEditor
              amountMinor={amountMinor}
              currency={currency}
              items={items}
              members={members}
              onChange={(nextItems) => {
                setItems(nextItems);
                setDirty(true);
              }}
            />
          ) : null}
          <FormMessage>{submit.error}</FormMessage>
          <FormMessage tone="success">{submit.feedback}</FormMessage>
        </form>
      </Container>
      <StickyActionBar>
        <Button
          disabled={!submit.canSubmit}
          form="expense-form"
          loading={submit.submitting}
          size="medium"
          type="submit"
        >
          {submitLabel}
        </Button>
      </StickyActionBar>
    </div>
  );
}
