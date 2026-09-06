import { Button, Container, Flex, Spinner, Typography } from '@maxhub/max-ui';
import type { ReactNode } from 'react';

interface EmptyStateProps {
  action?: ReactNode;
  description: string;
  title: string;
}

export function EmptyState({ action, description, title }: EmptyStateProps) {
  return (
    <Container className="state-view">
      <Flex align="center" direction="column" gap={12}>
        <div aria-hidden="true" className="state-view__symbol">
          ···
        </div>
        <Typography.Headline asChild variant="medium">
          <h3>{title}</h3>
        </Typography.Headline>
        <Typography.Body asChild color="secondary">
          <p>{description}</p>
        </Typography.Body>
        {action}
      </Flex>
    </Container>
  );
}

interface LoadingStateProps {
  label?: string;
}

export function LoadingState({ label = 'Загружаем данные…' }: LoadingStateProps) {
  return (
    <Container aria-live="polite" className="state-view" role="status">
      <Flex align="center" direction="column" gap={12}>
        <Spinner aria-hidden="true" size={32} />
        <Typography.Body color="secondary">{label}</Typography.Body>
      </Flex>
    </Container>
  );
}

interface ErrorStateProps {
  description?: string;
  onRetry?: () => void;
  title?: string;
}

export function ErrorState({
  description,
  onRetry,
  title = 'Что-то пошло не так',
}: ErrorStateProps) {
  return (
    <Container aria-live="assertive" className="state-view" role="alert">
      <Flex align="center" direction="column" gap={12}>
        <div aria-hidden="true" className="state-view__symbol state-view__symbol--error">
          !
        </div>
        <Typography.Headline asChild variant="medium">
          <h3>{title}</h3>
        </Typography.Headline>
        {description ? (
          <Typography.Body asChild color="secondary">
            <p>{description}</p>
          </Typography.Body>
        ) : null}
        {onRetry ? (
          <Button onClick={onRetry} size="small" variant="secondary">
            Повторить
          </Button>
        ) : null}
      </Flex>
    </Container>
  );
}
