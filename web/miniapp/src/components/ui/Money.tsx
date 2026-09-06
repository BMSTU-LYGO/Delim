import type { HTMLAttributes } from 'react';

interface MoneyProps extends HTMLAttributes<HTMLSpanElement> {
  amountMinor: number;
  currency?: string;
  sign?: MoneySign;
}

type MoneySign = NonNullable<Intl.NumberFormatOptions['signDisplay']>;

const formatters = new Map<string, Intl.NumberFormat>();

export const formatMoney = (amountMinor: number, currency = 'RUB', sign: MoneySign = 'auto') => {
  const key = `${currency}:${sign}`;
  let formatter = formatters.get(key);
  if (!formatter) {
    formatter = new Intl.NumberFormat('ru-RU', {
      currency,
      currencyDisplay: 'symbol',
      signDisplay: sign,
      style: 'currency',
    });
    formatters.set(key, formatter);
  }
  const digits = formatter.resolvedOptions().maximumFractionDigits ?? 2;
  return formatter.format(amountMinor / 10 ** digits);
};

export function Money({ amountMinor, currency = 'RUB', sign = 'auto', ...props }: MoneyProps) {
  return <span {...props}>{formatMoney(amountMinor, currency, sign)}</span>;
}
