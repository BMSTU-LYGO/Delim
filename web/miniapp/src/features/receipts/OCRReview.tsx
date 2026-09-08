import { Button, Flex, Input, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import type { GroupMember, OCRResult, Receipt } from '../../api';
import { userErrorMessage } from '../../api';
import { FormField, FormMessage, useDirtyForm } from '../../components/form';
import { StatusBadge } from '../../components/ui';
import { moneyInputFromMinor, parseMoneyInput } from '../../domain/money';
import {
  createDraftItem,
  ItemSplitEditor,
  itemSplitValidation,
  type ExpenseDraftItem,
} from '../expenses/ItemSplitEditor';
import type { ExpenseFormPrefill } from '../expenses/ExpenseForm';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

const localDateTime = (value?: string) => {
  const source = value ? new Date(value) : new Date();
  const local = new Date(source.getTime() - source.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
};

const confidenceLabel = (confidence: number) => {
  if (confidence < 0.65) return { label: 'Нужно внимательно проверить', tone: 'warning' as const };
  if (confidence < 0.85) return { label: 'Стоит проверить', tone: 'warning' as const };
  return { label: 'Распознано уверенно', tone: 'positive' as const };
};

interface OCRReviewProps {
  ocr: OCRResult;
  receipt: Receipt;
}

export function OCRReview({ ocr, receipt }: OCRReviewProps) {
  const { client } = useSession();
  const navigate = useNavigate();
  const [members, setMembers] = useState<GroupMember[]>();
  const [membersError, setMembersError] = useState<string>();
  const [merchant, setMerchant] = useState(ocr.merchant ?? '');
  const [date, setDate] = useState(() => localDateTime(ocr.date));
  const [currency, setCurrency] = useState(ocr.currency ?? 'RUB');
  const initialTotal = ocr.total_minor ?? ocr.items.reduce((sum, item) => sum + item.amount_minor, 0);
  const [total, setTotal] = useState(() => moneyInputFromMinor(initialTotal, ocr.currency ?? 'RUB'));
  const [items, setItems] = useState<ExpenseDraftItem[]>(() =>
    ocr.items.map((item) => ({
      ...createDraftItem(item.amount_minor, ocr.currency ?? 'RUB'),
      confidence: item.confidence,
      name: item.name,
    })),
  );
  const [dirty, setDirty] = useState(false);

  useDirtyForm(dirty);

  const loadMembers = useCallback(
    async (signal?: AbortSignal) => {
      setMembersError(undefined);
      try {
        setMembers(await client.listGroupMembers(receipt.group_id, signal));
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setMembersError(userErrorMessage(cause, 'Не удалось загрузить участников'));
      }
    },
    [client, receipt.group_id],
  );

  useEffect(() => {
    const controller = new AbortController();
    void loadMembers(controller.signal);
    return () => controller.abort();
  }, [loadMembers]);

  const totalMinor = useMemo(() => parseMoneyInput(total, currency), [currency, total]);
  const totalError = totalMinor === undefined || totalMinor <= 0 ? 'Укажите положительную общую сумму' : undefined;
  const currencyError = /^[A-Z]{3}$/.test(currency) ? undefined : 'Введите код из трёх букв';
  const itemsError = members
    ? itemSplitValidation(items, totalMinor, currency)
    : 'Дождитесь загрузки участников';
  const validDate = !Number.isNaN(new Date(date).getTime());
  const isValid = !totalError && !currencyError && !itemsError && validDate;
  const confidence = confidenceLabel(ocr.confidence);

  const openExpenseForm = () => {
    if (!isValid || totalMinor === undefined) return;
    const prefill: ExpenseFormPrefill = {
      amountMinor: totalMinor,
      currency,
      description: merchant.trim() || 'Расход по чеку',
      expenseDate: new Date(date).toISOString(),
      items: items.map((item) => ({
        amountMinor: parseMoneyInput(item.amount, currency) ?? 0,
        name: item.name.trim(),
        participantIds: item.participantIds,
      })),
    };
    navigate(routes.newExpense(String(receipt.group_id)), { state: { prefill, receiptId: receipt.id } });
  };

  return (
    <section aria-labelledby="ocr-review-title" className="ocr-review">
      <Flex direction="column" gap={20}>
        <div className="ocr-review__heading">
          <div>
            <Typography.Headline asChild variant="medium">
              <h2 id="ocr-review-title">Проверьте чек</h2>
            </Typography.Headline>
            <Typography.Body color="secondary" variant="small">
              Исправьте распознавание и назначьте участников каждой позиции.
            </Typography.Body>
          </div>
          <Flex gap={6} wrap="wrap">
            <StatusBadge tone={confidence.tone}>{confidence.label}</StatusBadge>
            {ocr.qr_found ? <StatusBadge tone="themed">QR найден</StatusBadge> : null}
          </Flex>
        </div>

        <div className="ocr-review__fields">
          <FormField htmlFor="ocr-merchant" label="Магазин или место">
            <Input
              id="ocr-merchant"
              onChange={(event) => {
                setMerchant(event.target.value);
                setDirty(true);
              }}
              placeholder="Название"
              value={merchant}
            />
          </FormField>
          <FormField htmlFor="ocr-date" label="Дата и время" required>
            <Input
              id="ocr-date"
              onChange={(event) => {
                setDate(event.target.value);
                setDirty(true);
              }}
              type="datetime-local"
              value={date}
            />
          </FormField>
          <div className="expense-form__money-row">
            <FormField error={totalError} htmlFor="ocr-total" label="Итого" required>
              <Input
                aria-invalid={Boolean(totalError)}
                id="ocr-total"
                inputMode="decimal"
                onChange={(event) => {
                  setTotal(event.target.value);
                  setDirty(true);
                }}
                value={total}
              />
            </FormField>
            <FormField error={currencyError} htmlFor="ocr-currency" label="Валюта" required>
              <Input
                aria-invalid={Boolean(currencyError)}
                id="ocr-currency"
                maxLength={3}
                onChange={(event) => {
                  setCurrency(event.target.value.toLocaleUpperCase('en-US'));
                  setDirty(true);
                }}
                value={currency}
              />
            </FormField>
          </div>
        </div>

        {members ? (
          <ItemSplitEditor
            amountMinor={totalMinor}
            currency={currency}
            items={items}
            members={members}
            onChange={(nextItems) => {
              setItems(nextItems);
              setDirty(true);
            }}
          />
        ) : (
          <Typography.Body color="secondary">Загружаем участников…</Typography.Body>
        )}
        <FormMessage>{membersError}</FormMessage>
        {membersError ? (
          <Button onClick={() => void loadMembers()} size="small" variant="secondary">
            Повторить
          </Button>
        ) : null}

        <div className="ocr-review__financial-note">
          <Typography.Body color="secondary" variant="small">
            Продолжение только заполнит форму расхода. Баланс изменится после отдельного финального
            сохранения на следующем экране.
          </Typography.Body>
        </div>
        <Button disabled={!isValid} onClick={openExpenseForm} size="medium" stretched>
          Продолжить к расходу
        </Button>
      </Flex>
    </section>
  );
}
