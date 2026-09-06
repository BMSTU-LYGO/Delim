import { Button, Input, Typography } from '@maxhub/max-ui';

import type { ExpenseItemInput, GroupMember, User } from '../../api';
import { FormMessage } from '../../components/form';
import { Money, StatusBadge } from '../../components/ui';
import { moneyInputFromMinor, parseMoneyInput } from '../../domain/money';

export interface ExpenseDraftItem {
  amount: string;
  clientId: string;
  name: string;
  participantIds: number[];
}

let nextItemId = 1;

export const createDraftItem = (
  amountMinor?: number,
  currency = 'RUB',
  participantIds: number[] = [],
): ExpenseDraftItem => ({
  amount: amountMinor === undefined ? '' : moneyInputFromMinor(amountMinor, currency),
  clientId: `item-${nextItemId++}`,
  name: '',
  participantIds: [...participantIds],
});

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

export function itemSplitValidation(
  items: ExpenseDraftItem[],
  amountMinor: number | undefined,
  currency: string,
): string | undefined {
  if (!items.length) return 'Добавьте хотя бы одну позицию';
  if (items.some((item) => !item.name.trim())) return 'Укажите название каждой позиции';
  const amounts = items.map((item) => parseMoneyInput(item.amount, currency));
  if (amounts.some((amount) => amount === undefined || amount <= 0)) {
    return 'Укажите положительную сумму каждой позиции';
  }
  if (items.some((item) => item.participantIds.length === 0)) {
    return 'Назначьте участников для каждой позиции';
  }
  if (amountMinor === undefined) return 'Сначала укажите общую сумму расхода';
  const total = amounts.reduce((sum, amount) => sum + BigInt(amount!), 0n);
  if (total !== BigInt(amountMinor)) {
    return total < BigInt(amountMinor) ? 'Сумма позиций меньше общей суммы' : 'Сумма позиций больше общей суммы';
  }
  return undefined;
}

export function buildExpenseItems(
  items: ExpenseDraftItem[],
  currency: string,
): ExpenseItemInput[] {
  return items.map((item) => ({
    amount_minor: parseMoneyInput(item.amount, currency) ?? 0,
    name: item.name.trim(),
    participant_user_ids: item.participantIds,
  }));
}

interface ItemSplitEditorProps {
  amountMinor?: number;
  currency: string;
  items: ExpenseDraftItem[];
  members: GroupMember[];
  onChange(items: ExpenseDraftItem[]): void;
}

export function ItemSplitEditor({
  amountMinor,
  currency,
  items,
  members,
  onChange,
}: ItemSplitEditorProps) {
  const update = (clientId: string, patch: Partial<ExpenseDraftItem>) =>
    onChange(items.map((item) => (item.clientId === clientId ? { ...item, ...patch } : item)));
  const allocatedMinor = items.reduce(
    (sum, item) => sum + (parseMoneyInput(item.amount, currency) ?? 0),
    0,
  );
  const unassignedCount = items.filter((item) => item.participantIds.length === 0).length;
  const validationError = itemSplitValidation(items, amountMinor, currency);

  return (
    <section aria-labelledby="items-title" className="item-editor">
      <div className="item-editor__heading">
        <Typography.Headline asChild variant="small">
          <h3 id="items-title">Позиции</h3>
        </Typography.Headline>
        {unassignedCount ? (
          <StatusBadge tone="warning">Не распределено: {unassignedCount}</StatusBadge>
        ) : null}
      </div>
      <div className="item-editor__list">
        {items.map((item, index) => (
          <fieldset className="item-editor__item" key={item.clientId}>
            <legend>Позиция {index + 1}</legend>
            <div className="item-editor__fields">
              <Input
                aria-label={`Название позиции ${index + 1}`}
                onChange={(event) => update(item.clientId, { name: event.target.value })}
                placeholder="Название"
                value={item.name}
              />
              <Input
                aria-label={`Сумма позиции ${index + 1}`}
                inputMode="decimal"
                onChange={(event) => update(item.clientId, { amount: event.target.value })}
                placeholder="0,00"
                value={item.amount}
              />
            </div>
            <Typography.Label asChild variant="medium-strong">
              <span>Кто делит позицию</span>
            </Typography.Label>
            <div className="item-editor__participants">
              {members.map((member) => {
                const checked = item.participantIds.includes(member.user_id);
                return (
                  <label className="participant-chip" key={member.user_id}>
                    <input
                      checked={checked}
                      onChange={() =>
                        update(item.clientId, {
                          participantIds: checked
                            ? item.participantIds.filter((id) => id !== member.user_id)
                            : [...item.participantIds, member.user_id],
                        })
                      }
                      type="checkbox"
                    />
                    <span>{userName(member)}</span>
                  </label>
                );
              })}
            </div>
            <Button
              aria-label={`Удалить позицию ${index + 1}`}
              onClick={() => onChange(items.filter((current) => current.clientId !== item.clientId))}
              size="xsmall"
              type="button"
              variant="ghost"
            >
              Удалить
            </Button>
          </fieldset>
        ))}
      </div>
      <Button
        onClick={() => onChange([...items, createDraftItem()])}
        size="small"
        type="button"
        variant="secondary"
      >
        Добавить позицию
      </Button>
      {amountMinor !== undefined ? (
        <Typography.Body color="secondary" variant="small">
          Позиции: <Money amountMinor={allocatedMinor} currency={currency} /> из{' '}
          <Money amountMinor={amountMinor} currency={currency} />
        </Typography.Body>
      ) : null}
      <FormMessage>{validationError}</FormMessage>
    </section>
  );
}
