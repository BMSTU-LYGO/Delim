const currencyDigits = (currency: string) => {
  try {
    return new Intl.NumberFormat('ru-RU', { currency, style: 'currency' }).resolvedOptions()
      .maximumFractionDigits ?? 2;
  } catch {
    return 2;
  }
};

export function parseMoneyInput(value: string, currency: string): number | undefined {
  const normalized = value.trim().replace(/[\s\u00a0]/g, '');
  const digits = currencyDigits(currency);
  if (digits === 0) {
    if (!/^\d+$/.test(normalized)) return undefined;
    const minor = BigInt(normalized);
    return minor <= BigInt(Number.MAX_SAFE_INTEGER) ? Number(minor) : undefined;
  }
  const match = normalized.match(new RegExp(`^(\\d+)(?:[,.](\\d{0,${digits}}))?$`));
  if (!match) return undefined;

  const [, major, fraction = ''] = match;
  const minor = BigInt(major) * 10n ** BigInt(digits) + BigInt(fraction.padEnd(digits, '0'));
  if (minor > BigInt(Number.MAX_SAFE_INTEGER)) return undefined;
  return Number(minor);
}

export function moneyInputFromMinor(amountMinor: number, currency: string): string {
  const digits = currencyDigits(currency);
  const divisor = 10 ** digits;
  const major = Math.trunc(amountMinor / divisor);
  if (digits === 0) return String(major);
  return `${major},${String(Math.abs(amountMinor % divisor)).padStart(digits, '0')}`;
}
