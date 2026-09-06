import { Container, Flex, Panel, Typography } from '@maxhub/max-ui';

export function App() {
  return (
    <Panel className="app" mode="secondary">
      <Container className="app__intro">
        <Flex direction="column" gap={8}>
          <Typography.Headline asChild variant="large-strong">
            <h1>Делим</h1>
          </Typography.Headline>
          <Typography.Body asChild color="secondary">
            <p>Совместные расходы без лишних расчётов.</p>
          </Typography.Body>
        </Flex>
      </Container>
    </Panel>
  );
}
