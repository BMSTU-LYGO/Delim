import { Flex, Typography } from '@maxhub/max-ui';
import type { ReactNode } from 'react';

interface FormFieldProps {
  children: ReactNode;
  error?: string;
  hint?: string;
  htmlFor?: string;
  label: string;
  required?: boolean;
}

export function FormField({
  children,
  error,
  hint,
  htmlFor,
  label,
  required = false,
}: FormFieldProps) {
  const message = error ?? hint;
  return (
    <Flex className="form-field" direction="column" gap={6}>
      <Typography.Label asChild variant="large-strong">
        <label htmlFor={htmlFor}>
          {label}
          {required ? <span aria-hidden="true"> *</span> : null}
        </label>
      </Typography.Label>
      {children}
      {message ? (
        <Typography.Body
          aria-live={error ? 'polite' : undefined}
          className={error ? 'form-field__error' : undefined}
          color={error ? 'inherit' : 'tertiary'}
          id={htmlFor ? `${htmlFor}-message` : undefined}
          variant="small"
        >
          {message}
        </Typography.Body>
      ) : null}
    </Flex>
  );
}

interface FormMessageProps {
  children?: ReactNode;
  tone?: 'error' | 'success';
}

export function FormMessage({ children, tone = 'error' }: FormMessageProps) {
  if (!children) return null;
  return (
    <div
      aria-live={tone === 'error' ? 'assertive' : 'polite'}
      className={`form-message form-message--${tone}`}
      role={tone === 'error' ? 'alert' : 'status'}
    >
      <Typography.Body>{children}</Typography.Body>
    </div>
  );
}
