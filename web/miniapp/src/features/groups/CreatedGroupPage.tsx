import { Button, Container, Flex, Typography } from '@maxhub/max-ui';
import { Link, useLocation, useParams } from 'react-router-dom';

import { PageHeader } from '../../components/ui';
import { routes } from '../../app/routes';

export function CreatedGroupPage() {
  const { groupId = '' } = useParams();
  const location = useLocation();
  const created = Boolean((location.state as { created?: boolean } | null)?.created);

  return (
    <div className="screen">
      <PageHeader title="Группа" />
      <Container>
        <Flex className="group-created" direction="column" gap={16}>
          <Typography.Body color="secondary">
            {created ? 'Группа создана. Теперь добавьте тех, с кем будете делить расходы.' : 'Загружаем данные группы.'}
          </Typography.Body>
          {created ? (
            <Button asChild size="medium">
              <Link to={routes.members(groupId)}>Пригласить участников</Link>
            </Button>
          ) : null}
        </Flex>
      </Container>
    </div>
  );
}
