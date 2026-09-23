import { Flex, Typography } from '@maxhub/max-ui';
import type { ReactNode } from 'react';

interface FormFieldProps {
  children: ReactNode;
  className?: string;
  error?: string;
  hint?: string;
  htmlFor?: string;
  label: string;
  required?: boolean;
  reserveMessage?: boolean;
}

export function FormField({
  children,
  className,
  error,
  hint,
  htmlFor,
  label,
  required = false,
  reserveMessage = false,
}: FormFieldProps) {
  const message = error ?? hint;
  return (
    <Flex className={`form-field${className ? ` ${className}` : ''}`} direction="column" gap={6}>
      <Typography.Label asChild variant="large-strong">
        <label htmlFor={htmlFor}>
          {label}
          {required ? <span aria-hidden="true"> *</span> : null}
        </label>
      </Typography.Label>
      {children}
      {(message || reserveMessage) ? (
        <Typography.Body
          aria-live={error ? 'polite' : undefined}
          className={`form-field__message${error ? ' form-field__error' : ''}`}
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
