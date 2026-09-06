import { useCallback, useRef, useState } from 'react';
import type { FormEvent } from 'react';

import { ApiError } from '../../api';

interface FormSubmitOptions<TResult> {
  isValid: boolean;
  onSubmit(): Promise<TResult>;
  onSuccess?(result: TResult): void | Promise<void>;
  successMessage?: string;
}

const errorMessage = (cause: unknown) => {
  if (cause instanceof ApiError) {
    if (cause.isConflict) return 'Данные уже изменились. Обновите страницу и попробуйте ещё раз.';
    return cause.message;
  }
  if (cause instanceof Error && cause.name === 'AbortError') return undefined;
  return 'Не удалось сохранить изменения. Проверьте подключение и повторите попытку.';
};

export function useFormSubmit<TResult>({
  isValid,
  onSubmit,
  onSuccess,
  successMessage = 'Готово',
}: FormSubmitOptions<TResult>) {
  const inFlight = useRef(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>();
  const [feedback, setFeedback] = useState<string>();

  const handleSubmit = useCallback(
    async (event?: FormEvent<HTMLFormElement>) => {
      event?.preventDefault();
      if (!isValid || inFlight.current) return;

      inFlight.current = true;
      setSubmitting(true);
      setError(undefined);
      setFeedback(undefined);
      try {
        const result = await onSubmit();
        setFeedback(successMessage);
        await onSuccess?.(result);
      } catch (cause) {
        setError(errorMessage(cause));
      } finally {
        inFlight.current = false;
        setSubmitting(false);
      }
    },
    [isValid, onSubmit, onSuccess, successMessage],
  );

  return {
    canSubmit: isValid && !submitting,
    clearError: () => setError(undefined),
    error,
    feedback,
    handleSubmit,
    submitting,
  };
}
