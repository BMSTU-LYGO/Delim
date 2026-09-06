import { Button, Container, Input } from '@maxhub/max-ui';
import { useCallback, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import type { Group } from '../../api';
import { FormField, FormMessage, useDirtyForm, useFormSubmit } from '../../components/form';
import { PageHeader, StickyActionBar } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

const maxNameLength = 120;

export function CreateGroupPage() {
  const { client } = useSession();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [touched, setTouched] = useState(false);
  const [committed, setCommitted] = useState(false);
  const normalizedName = name.trim();
  const nameError = !normalizedName
    ? 'Введите название группы'
    : name.length > maxNameLength
      ? `Не больше ${maxNameLength} символов`
      : undefined;

  useDirtyForm(name.length > 0 && !committed);

  const createGroup = useCallback(
    () => client.createGroup(normalizedName),
    [client, normalizedName],
  );
  const openGroup = useCallback(
    (group: Group) => {
      setCommitted(true);
      navigate(routes.group(String(group.id)), { replace: true, state: { created: true } });
    },
    [navigate],
  );
  const submit = useFormSubmit({
    isValid: !nameError,
    onSubmit: createGroup,
    onSuccess: openGroup,
    successMessage: 'Группа создана',
  });

  return (
    <div className="screen create-group-page">
      <PageHeader
        subtitle="Например, «Поездка в Казань» или «Квартира»"
        title="Новая группа"
      />
      <Container className="create-group-page__content">
        <form className="form-stack" id="create-group-form" onSubmit={submit.handleSubmit}>
          <FormField
            error={touched ? nameError : undefined}
            htmlFor="group-name"
            label="Название"
            required
          >
            <Input
              aria-describedby="group-name-message"
              aria-invalid={touched && Boolean(nameError)}
              autoComplete="off"
              autoFocus
              id="group-name"
              maxLength={maxNameLength + 1}
              onBlur={() => setTouched(true)}
              onChange={(event) => setName(event.target.value)}
              placeholder="Название группы"
              value={name}
              withClearButton
            />
          </FormField>
          <FormMessage>{submit.error}</FormMessage>
          <FormMessage tone="success">{submit.feedback}</FormMessage>
        </form>
      </Container>
      <StickyActionBar>
        <Button
          disabled={!submit.canSubmit}
          form="create-group-form"
          loading={submit.submitting}
          size="medium"
          type="submit"
        >
          Создать группу
        </Button>
      </StickyActionBar>
    </div>
  );
}
