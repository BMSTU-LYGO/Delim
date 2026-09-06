import { Button, Container, Flex, Input, Typography } from '@maxhub/max-ui';
import { useCallback, useMemo, useState } from 'react';

import type { Expense, ExpenseInput, Group, GroupMember, SplitType, User } from '../../api';
import { FormField, FormMessage, useDirtyForm, useFormSubmit } from '../../components/form';
import { PageHeader, StickyActionBar, UserAvatar } from '../../components/ui';
import { parseMoneyInput } from '../../domain/money';
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
  members: GroupMember[];
  onSave(input: ExpenseInput): Promise<Expense>;
  onSaved(expense: Expense): void;
}

export function ExpenseForm({ group, members, onSave, onSaved }: ExpenseFormProps) {
  const [description, setDescription] = useState('');
  const [amount, setAmount] = useState('');
  const [currency, setCurrency] = useState('RUB');
  const [payerId, setPayerId] = useState(group.owner_id);
  const [initialExpenseDate] = useState(localDateTime);
  const [expenseDate, setExpenseDate] = useState(initialExpenseDate);
  const [splitType, setSplitType] = useState<SplitType>('equal');
  const [splitValues, setSplitValues] = useState<SplitValues>({});
  const [items, setItems] = useState<ExpenseDraftItem[]>([]);
  const [participantIds, setParticipantIds] = useState<number[]>(() =>
    members.map((member) => member.user_id),
  );
  const [touched, setTouched] = useState<Record<string, boolean>>({});
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

  const dirty = Boolean(
    description ||
      amount ||
      currency !== 'RUB' ||
      payerId !== group.owner_id ||
      expenseDate !== initialExpenseDate ||
      participantIds.length !== members.length ||
      splitType !== 'equal',
  );
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
    onSubmit: submitExpense,
    onSuccess: finish,
    successMessage: 'Расход сохранён',
  });

  const touch = (field: string) => setTouched((current) => ({ ...current, [field]: true }));
  const toggleParticipant = (userId: number) => {
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
    setSplitType(next);
    setSplitValues(defaultSplitValues(next, participantIds, amountMinor, currency));
    if (next === 'item' && items.length === 0) {
      setItems([createDraftItem(amountMinor, currency, participantIds)]);
    }
  };

  return (
    <div className="screen expense-form-page">
      <PageHeader subtitle={group.name} title="Новый расход" />
      <Container className="expense-form-page__content">
        <form className="form-stack" id="expense-form" onSubmit={submit.handleSubmit}>
          <FormField htmlFor="expense-description" label="Описание">
            <Input
              autoComplete="off"
              id="expense-description"
              maxLength={240}
              onChange={(event) => setDescription(event.target.value)}
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
                onChange={(event) => setAmount(event.target.value)}
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
                onChange={(event) => setCurrency(event.target.value.toLocaleUpperCase('en-US'))}
                value={currency}
              />
            </FormField>
          </div>

          <FormField htmlFor="expense-payer" label="Кто заплатил" required>
            <select
              className="native-select"
              id="expense-payer"
              onChange={(event) => setPayerId(Number(event.target.value))}
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
              onChange={(event) => setExpenseDate(event.target.value)}
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
            onChange={(userId, value) =>
              setSplitValues((current) => ({ ...current, [userId]: value }))
            }
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
              onChange={setItems}
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
          Сохранить расход
        </Button>
      </StickyActionBar>
    </div>
  );
}
