import { Button, Container, Input } from '@maxhub/max-ui';
import { useCallback, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import type { CreateGroupInput, Group, GroupActivityType } from '../../api';
import { FormField, FormMessage, useDirtyForm, useFormSubmit } from '../../components/form';
import { PageHeader, StickyActionBar } from '../../components/ui';
import { parseMoneyInput } from '../../domain/money';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

const maxNameLength = 120;
const activityLabels: Record<Exclude<GroupActivityType, ''>, string> = {
  trip: 'Поездка',
  hike: 'Поход',
  event: 'Событие',
};

export function CreateGroupPage() {
  const { client } = useSession();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [activityType, setActivityType] = useState<GroupActivityType>('event');
  const [location, setLocation] = useState('');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');
  const [budget, setBudget] = useState('');
  const [touched, setTouched] = useState(false);
  const [committed, setCommitted] = useState(false);
  const normalizedName = name.trim();
  const nameError = !normalizedName
    ? 'Введите название группы'
    : name.length > maxNameLength
      ? `Не больше ${maxNameLength} символов`
      : undefined;
  const budgetMinor = budget.trim() ? parseMoneyInput(budget, 'RUB') : undefined;
  const detailsError = (startDate && !endDate) || (!startDate && endDate)
    ? 'Укажите обе даты или оставьте их пустыми'
    : startDate && endDate && endDate < startDate
      ? 'Дата окончания раньше даты начала'
      : budget.trim() && (budgetMinor === undefined || budgetMinor < 0)
        ? 'Укажите бюджет в рублях'
        : undefined;

  useDirtyForm((name.length > 0 || location.length > 0 || budget.length > 0 || Boolean(startDate)) && !committed);

  const createGroup = useCallback(
    () => {
      const input: CreateGroupInput = {
        activity_type: activityType,
        name: normalizedName,
      };
      if (location.trim()) input.location = location.trim();
      if (startDate) input.start_date = startDate;
      if (endDate) input.end_date = endDate;
      if (budgetMinor !== undefined) input.planned_budget_minor = budgetMinor;
      return client.createGroup(input);
    },
    [activityType, budgetMinor, client, endDate, location, normalizedName, startDate],
  );
  const openGroup = useCallback(
    (group: Group) => {
      setCommitted(true);
      navigate(routes.group(String(group.id)), { replace: true, state: { created: true } });
    },
    [navigate],
  );
  const submit = useFormSubmit({
    isValid: !nameError && !detailsError,
    onSubmit: createGroup,
    onSuccess: openGroup,
    successMessage: 'Группа создана',
  });

  return (
    <div className="screen create-group-page">
      <PageHeader
        subtitle="Один план — все траты и расчёты с друзьями в одном месте"
        title="Создать план"
      />
      <Container className="create-group-page__content">
        <form className="form-stack" id="create-group-form" onSubmit={submit.handleSubmit}>
          <FormField
            className="create-group-page__title"
            error={touched ? nameError : undefined}
            htmlFor="group-name"
            hint="Например: Поездка в Казань, Уикенд за городом"
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
              placeholder="Например, уикенд в Суздале"
              required
              value={name}
              withClearButton
            />
          </FormField>
          <FormField htmlFor="activity-type" label="Формат плана" required>
            <select
              className="native-select"
              id="activity-type"
              onChange={(event) => setActivityType(event.target.value as GroupActivityType)}
              value={activityType}
            >
              {(Object.entries(activityLabels) as Array<[Exclude<GroupActivityType, ''>, string]>).map(([value, label]) => (
                <option key={value} value={value}>{label}</option>
              ))}
            </select>
          </FormField>
          <FormField htmlFor="activity-location" label="Где" >
            <Input id="activity-location" maxLength={120} onChange={(event) => setLocation(event.target.value)} placeholder="Например, Санкт-Петербург" value={location} />
          </FormField>
          <div className="create-group-page__dates">
            <FormField htmlFor="activity-start" label="Начало">
              <Input
                aria-describedby="activity-budget-message"
                aria-invalid={touched && Boolean(detailsError)}
                id="activity-start"
                onBlur={() => setTouched(true)}
                onChange={(event) => setStartDate(event.target.value)}
                type="date"
                value={startDate}
              />
            </FormField>
            <FormField htmlFor="activity-end" label="Окончание">
              <Input
                aria-describedby="activity-budget-message"
                aria-invalid={touched && Boolean(detailsError)}
                id="activity-end"
                min={startDate || undefined}
                onBlur={() => setTouched(true)}
                onChange={(event) => setEndDate(event.target.value)}
                type="date"
                value={endDate}
              />
            </FormField>
          </div>
          <FormField error={touched ? detailsError : undefined} htmlFor="activity-budget" label="Бюджет плана, ₽">
            <Input
              aria-describedby="activity-budget-message"
              aria-invalid={touched && Boolean(detailsError)}
              id="activity-budget"
              inputMode="decimal"
              onBlur={() => setTouched(true)}
              onChange={(event) => setBudget(event.target.value)}
              placeholder="Необязательно"
              value={budget}
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
          Создать план
        </Button>
      </StickyActionBar>
    </div>
  );
}
