import { Container, Flex, Typography } from '@maxhub/max-ui';
import type { ReactNode } from 'react';

interface PageHeaderProps {
  action?: ReactNode;
  subtitle?: ReactNode;
  title: ReactNode;
}

export function PageHeader({ action, subtitle, title }: PageHeaderProps) {
  return (
    <Container className="page-header">
      <Flex align="center" gap={16} justify="space-between">
        <Flex direction="column" gap={4}>
          <Typography.Headline asChild variant="large-strong">
            <h2>{title}</h2>
          </Typography.Headline>
          {subtitle ? (
            <Typography.Body asChild color="secondary" variant="medium">
              <p>{subtitle}</p>
            </Typography.Body>
          ) : null}
        </Flex>
        {action}
      </Flex>
    </Container>
  );
}
