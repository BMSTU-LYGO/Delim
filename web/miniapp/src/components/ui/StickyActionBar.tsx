import { Container, Flex } from '@maxhub/max-ui';
import type { ReactNode } from 'react';

interface StickyActionBarProps {
  children: ReactNode;
}

export function StickyActionBar({ children }: StickyActionBarProps) {
  return (
    <div className="sticky-action-bar">
      <Container>
        <Flex className="sticky-action-bar__content" gap={8} wrap="wrap">
          {children}
        </Flex>
      </Container>
    </div>
  );
}
