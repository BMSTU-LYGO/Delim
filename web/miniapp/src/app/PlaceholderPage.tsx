import { Container, Flex, Typography } from '@maxhub/max-ui';

interface PlaceholderPageProps {
  title: string;
}

export function PlaceholderPage({ title }: PlaceholderPageProps) {
  return (
    <Container className="placeholder-page">
      <Flex direction="column" gap={8}>
        <Typography.Headline asChild variant="large-strong">
          <h2>{title}</h2>
        </Typography.Headline>
        <Typography.Body asChild color="secondary">
          <p>Экран готов к подключению данных.</p>
        </Typography.Body>
      </Flex>
    </Container>
  );
}
