import { Input, Typography } from '@maxhub/max-ui';

import type { GroupMember, SplitParticipant, SplitType, User } from '../../api';
import { Money } from '../../components/ui';
import { moneyInputFromMinor, parseMoneyInput } from '../../domain/money';

export type SplitValues = Record<number, string>;

const memberUser = (member: GroupMember): User =>
  member.user ?? {
    first_name: '',
    id: member.user_id,
    last_name: '',
    max_user_id: 0,
    username: `id${member.user_id}`,
  };

const memberName = (member: GroupMember) => {
  const user = memberUser(member);
  return [user.first_name, user.last_name].filter(Boolean).join(' ') || user.username;
};

const parsePositiveInteger = (value: string) => {
  if (!/^\d+$/.test(value)) return undefined;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined;
};

export const parsePercentage = (value: string): number | undefined => {
  const match = value.trim().match(/^(\d+)(?:[,.](\d{0,2}))?$/);
  if (!match) return undefined;
  const basisPoints = BigInt(match[1]) * 100n + BigInt((match[2] ?? '').padEnd(2, '0'));
  return basisPoints <= BigInt(Number.MAX_SAFE_INTEGER) ? Number(basisPoints) : undefined;
};

const percentageInput = (basisPoints: number) => {
  const major = Math.trunc(basisPoints / 100);
  const fraction = String(basisPoints % 100).padStart(2, '0').replace(/0+$/, '');
  return fraction ? `${major},${fraction}` : String(major);
};

const parsedValues = (
  splitType: SplitType,
  participantIds: number[],
  values: SplitValues,
  currency: string,
) =>
  participantIds.map((userId) => {
    const raw = values[userId] ?? '';
    const value = splitType === 'fixed'
      ? parseMoneyInput(raw, currency)
      : splitType === 'percentage'
        ? parsePercentage(raw)
        : parsePositiveInteger(raw);
    return { user_id: userId, value };
  });

export function splitValidation(
  splitType: SplitType,
  participantIds: number[],
  values: SplitValues,
  amountMinor: number | undefined,
  currency: string,
): string | undefined {
  if (!participantIds.length) return 'Выберите хотя бы одного участника';
  if (splitType === 'equal') return undefined;
  if (splitType === 'item') return 'Добавьте и распределите позиции';

  const parsed = parsedValues(splitType, participantIds, values, currency);
  if (parsed.some((participant) => participant.value === undefined)) {
    if (splitType === 'shares') return 'Для каждого участника укажите целое число долей больше нуля';
    if (splitType === 'percentage') return 'Для каждого участника укажите процент больше нуля';
    return 'Для каждого участника укажите корректную сумму';
  }
  if (splitType === 'percentage' && parsed.some((participant) => participant.value! <= 0)) {
    return 'Для каждого участника укажите процент больше нуля';
  }

  const total = parsed.reduce((sum, participant) => sum + BigInt(participant.value!), 0n);
  if (splitType === 'percentage' && total !== 10_000n) return 'Сумма процентов должна быть ровно 100%';
  if (splitType === 'fixed' && amountMinor !== undefined && total !== BigInt(amountMinor)) {
    return total < BigInt(amountMinor) ? 'Распределена не вся сумма' : 'Распределено больше общей суммы';
  }
  return undefined;
}

export function buildSplitParticipants(
  splitType: SplitType,
  participantIds: number[],
  values: SplitValues,
  currency: string,
): SplitParticipant[] {
  if (splitType === 'equal') return participantIds.map((userId) => ({ user_id: userId }));
  return parsedValues(splitType, participantIds, values, currency).map((participant) => ({
    user_id: participant.user_id,
    value: participant.value ?? 0,
  }));
}

export function defaultSplitValues(
  splitType: SplitType,
  participantIds: number[],
  amountMinor: number | undefined,
  currency: string,
): SplitValues {
  const ids = [...participantIds].sort((a, b) => a - b);
  if (splitType === 'shares') return Object.fromEntries(ids.map((id) => [id, '1']));
  if (splitType === 'percentage' && ids.length) {
    const base = Math.trunc(10_000 / ids.length);
    const remainder = 10_000 % ids.length;
    return Object.fromEntries(
      ids.map((id, index) => [id, percentageInput(base + (index < remainder ? 1 : 0))]),
    );
  }
  if (splitType === 'fixed' && ids.length && amountMinor !== undefined) {
    const base = Math.trunc(amountMinor / ids.length);
    const remainder = amountMinor % ids.length;
    return Object.fromEntries(
      ids.map((id, index) => [
        id,
        moneyInputFromMinor(base + (index < remainder ? 1 : 0), currency),
      ]),
    );
  }
  return {};
}

const allocationPreview = (
  splitType: SplitType,
  participantIds: number[],
  values: SplitValues,
  amountMinor: number,
  currency: string,
) => {
  const ids = [...participantIds].sort((a, b) => a - b);
  if (splitType === 'equal') {
    const base = Math.trunc(amountMinor / ids.length);
    const remainder = amountMinor % ids.length;
    return new Map(ids.map((id, index) => [id, base + (index < remainder ? 1 : 0)]));
  }
  if (splitType === 'fixed') {
    return new Map(ids.map((id) => [id, parseMoneyInput(values[id] ?? '', currency) ?? 0]));
  }
  const participants = parsedValues(splitType, ids, values, currency);
  if (participants.some((participant) => participant.value === undefined)) return new Map<number, number>();
  const total = participants.reduce((sum, participant) => sum + BigInt(participant.value!), 0n);
  if (total === 0n) return new Map<number, number>();
  let allocated = 0;
  const result = new Map<number, number>();
  participants.forEach((participant) => {
    const amount = Number((BigInt(amountMinor) * BigInt(participant.value!)) / total);
    result.set(participant.user_id, amount);
    allocated += amount;
  });
  for (let index = 0; index < amountMinor - allocated; index += 1) {
    const id = ids[index];
    if (id !== undefined) result.set(id, (result.get(id) ?? 0) + 1);
  }
  return result;
};

interface SplitModeEditorProps {
  amountMinor?: number;
  currency: string;
  members: GroupMember[];
  onChange(userId: number, value: string): void;
  participantIds: number[];
  splitType: SplitType;
  values: SplitValues;
}

export function SplitModeEditor({
  amountMinor,
  currency,
  members,
  onChange,
  participantIds,
  splitType,
  values,
}: SplitModeEditorProps) {
  if (splitType === 'item' || !participantIds.length) return null;
  const selectedMembers = members.filter((member) => participantIds.includes(member.user_id));
  const preview = amountMinor
    ? allocationPreview(splitType, participantIds, values, amountMinor, currency)
    : new Map<number, number>();
  const totalPercentage = splitType === 'percentage'
    ? participantIds.reduce((sum, id) => sum + (parsePercentage(values[id] ?? '') ?? 0), 0)
    : undefined;
  const fixedTotal = splitType === 'fixed'
    ? participantIds.reduce((sum, id) => sum + (parseMoneyInput(values[id] ?? '', currency) ?? 0), 0)
    : undefined;

  return (
    <div className="split-editor">
      <Typography.Headline asChild variant="small">
        <h3>Распределение</h3>
      </Typography.Headline>
      <div className="split-editor__rows">
        {selectedMembers.map((member) => (
          <div className="split-editor__row" key={member.user_id}>
            <span className="split-editor__name">{memberName(member)}</span>
            {splitType !== 'equal' ? (
              <Input
                aria-label={`${splitType === 'fixed' ? 'Сумма' : splitType === 'shares' ? 'Доли' : 'Процент'}: ${memberName(member)}`}
                className="split-editor__input"
                inputMode={splitType === 'shares' ? 'numeric' : 'decimal'}
                onChange={(event) => onChange(member.user_id, event.target.value)}
                placeholder={splitType === 'fixed' ? '0,00' : splitType === 'shares' ? '1' : '0'}
                value={values[member.user_id] ?? ''}
              />
            ) : null}
            <Typography.Body className="split-editor__preview" color="secondary" variant="medium">
              {preview.has(member.user_id) ? (
                <Money amountMinor={preview.get(member.user_id)!} currency={currency} />
              ) : (
                '—'
              )}
            </Typography.Body>
          </div>
        ))}
      </div>
      {totalPercentage !== undefined ? (
        <Typography.Body color={totalPercentage === 10_000 ? 'secondary' : 'inherit'} variant="small">
          Всего: {percentageInput(totalPercentage)}%
        </Typography.Body>
      ) : null}
      {fixedTotal !== undefined && amountMinor !== undefined ? (
        <Typography.Body color={fixedTotal === amountMinor ? 'secondary' : 'inherit'} variant="small">
          {fixedTotal <= amountMinor ? 'Осталось' : 'Перебор'}:{' '}
          <Money amountMinor={Math.abs(amountMinor - fixedTotal)} currency={currency} />
        </Typography.Body>
      ) : null}
      <Typography.Body color="tertiary" variant="small">
        Это предварительный расчёт. Итог проверит сервер при сохранении.
      </Typography.Body>
    </div>
  );
}
