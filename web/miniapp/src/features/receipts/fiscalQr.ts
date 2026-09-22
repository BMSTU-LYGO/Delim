export interface FiscalQr {
  date: string;
  time: string;
  sum: number;
  fn: string;
  fd: string;
  fp: string;
  operation: number;
}

const requiredFields = ['t', 's', 'fn', 'i', 'fp', 'n'] as const;

const isPositiveInteger = (value: string | undefined) =>
  value !== undefined && /^[1-9]\d*$/.test(value);

export const parseFiscalQr = (payload: string): FiscalQr | undefined => {
  const fields = new Map<string, string>();

  for (const part of payload.trim().split('&')) {
    const separator = part.indexOf('=');
    if (separator <= 0) return undefined;
    const key = part.slice(0, separator);
    const value = part.slice(separator + 1);
    if (fields.has(key)) return undefined;
    fields.set(key, value);
  }

  if (requiredFields.some((field) => !fields.get(field))) return undefined;

  const timestamp = fields.get('t')!;
  const timestampMatch = /^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})$/.exec(timestamp);
  const amount = fields.get('s')!;
  if (!timestampMatch || !/^\d+(?:\.\d{1,2})?$/.test(amount) || Number(amount) <= 0) return undefined;

  const [, year, month, day, hour, minute] = timestampMatch;
  const localDate = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute)));
  if (
    localDate.getUTCFullYear() !== Number(year) ||
    localDate.getUTCMonth() !== Number(month) - 1 ||
    localDate.getUTCDate() !== Number(day) ||
    localDate.getUTCHours() !== Number(hour) ||
    localDate.getUTCMinutes() !== Number(minute)
  ) {
    return undefined;
  }

  const fn = fields.get('fn');
  const fd = fields.get('i');
  const fp = fields.get('fp');
  const operation = fields.get('n');
  if (![fn, fd, fp, operation].every(isPositiveInteger)) return undefined;

  return {
    date: `${year}-${month}-${day}`,
    time: `${hour}:${minute}`,
    sum: Number(amount),
    fn: fn!,
    fd: fd!,
    fp: fp!,
    operation: Number(operation),
  };
};
