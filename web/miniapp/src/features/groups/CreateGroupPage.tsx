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
const maxSupportedDate = '9999-12-31';
const minSupportedDate = '0001-01-01';
const unsupportedDatePattern = /^\d{5,}-/;
const activityLabels: Record<Exclude<GroupActivityType, ''>, string> = {
  trip: 'Поездка',
  hike: 'Поход',
  event: 'Событие',
};
type GroupActivityFormType = Exclude<GroupActivityType, ''> | 'other';

export function CreateGroupPage() {
  const { client } = useSession();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [activityType, setActivityType] = useState<GroupActivityFormType>('event');
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
  const startDateError = !startDate && endDate ? 'Укажите дату начала' : undefined;
  const endDateError = startDate && !endDate
    ? 'Укажите дату окончания'
    : startDate && endDate && endDate < startDate
      ? 'Дата окончания раньше даты начала'
      : undefined;
  const datesError = startDateError ?? endDateError;
  const budgetError = budget.trim() && (budgetMinor === undefined || budgetMinor < 0)
    ? 'Укажите бюджет в рублях'
    : undefined;

  useDirtyForm(
    (name.length > 0 ||
      location.length > 0 ||
      budget.length > 0 ||
      Boolean(startDate) ||
      Boolean(endDate)) &&
      !committed,
  );

  const createGroup = useCallback(
    () => {
      const input: Omit<CreateGroupInput, 'activity_type'> & Partial<Pick<CreateGroupInput, 'activity_type'>> = {
        name: normalizedName,
      };
      if (activityType !== 'other') input.activity_type = activityType;
      if (location.trim()) input.location = location.trim();
      if (startDate) input.start_date = startDate;
      if (endDate) input.end_date = endDate;
      if (budgetMinor !== undefined) input.planned_budget_minor = budgetMinor;
      return client.createGroup(input as CreateGroupInput);
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
    isValid: !nameError && !datesError && !budgetError,
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
            reserveMessage
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
          <FormField className="create-group-page__location" htmlFor="activity-location" label="Где" reserveMessage>
            <Input id="activity-location" maxLength={120} onChange={(event) => setLocation(event.target.value)} placeholder="Например, Санкт-Петербург" value={location} />
          </FormField>
          <div className="create-group-page__dates">
            <FormField error={touched ? startDateError : undefined} htmlFor="activity-start" label="Начало" reserveMessage>
              <div className={`create-group-page__date-control ${startDate ? 'create-group-page__date-control--filled' : 'create-group-page__date-control--empty'}`}>
                <Input
                  aria-describedby="activity-start-message"
                  aria-invalid={touched && Boolean(startDateError)}
                  id="activity-start"
                  max={endDate || maxSupportedDate}
                  min={minSupportedDate}
                  onBlur={() => setTouched(true)}
                  onChange={(event) => {
                    const nextDate = event.currentTarget.value;
                    if (unsupportedDatePattern.test(nextDate)) {
                      event.currentTarget.value = startDate;
                      return;
                    }
                    setStartDate(nextDate);
                  }}
                  type="date"
                  value={startDate}
                />
                {!startDate ? <span aria-hidden="true" className="create-group-page__date-placeholder">ДД.ММ.ГГГГ</span> : null}
              </div>
            </FormField>
            <FormField error={touched ? endDateError : undefined} htmlFor="activity-end" label="Окончание" reserveMessage>
              <div className={`create-group-page__date-control ${endDate ? 'create-group-page__date-control--filled' : 'create-group-page__date-control--empty'}`}>
                <Input
                  aria-describedby="activity-end-message"
                  aria-invalid={touched && Boolean(endDateError)}
                  id="activity-end"
                  max={maxSupportedDate}
                  min={startDate || undefined}
                  onBlur={() => setTouched(true)}
                  onChange={(event) => {
                    const nextDate = event.currentTarget.value;
                    if (unsupportedDatePattern.test(nextDate)) {
                      event.currentTarget.value = endDate;
                      return;
                    }
                    setEndDate(nextDate);
                  }}
                  type="date"
                  value={endDate}
                />
                {!endDate ? <span aria-hidden="true" className="create-group-page__date-placeholder">ДД.ММ.ГГГГ</span> : null}
              </div>
            </FormField>
          </div>
          <div className="create-group-page__format-budget">
            <FormField htmlFor="activity-type" label="Формат плана" required reserveMessage>
              <select
                className="native-select"
                id="activity-type"
                onChange={(event) => setActivityType(event.target.value as GroupActivityFormType)}
                value={activityType}
              >
                {(Object.entries(activityLabels) as Array<[Exclude<GroupActivityType, ''>, string]>).map(([value, label]) => (
                  <option key={value} value={value}>{label}</option>
                ))}
                <option value="other">Другое</option>
              </select>
            </FormField>
            <FormField error={touched ? budgetError : undefined} htmlFor="activity-budget" label="Бюджет" reserveMessage>
              <Input
                aria-describedby="activity-budget-message"
                aria-invalid={touched && Boolean(budgetError)}
                id="activity-budget"
                inputMode="decimal"
                onBlur={() => setTouched(true)}
                onChange={(event) => setBudget(event.target.value)}
                placeholder="Необязательно"
                value={budget}
              />
            </FormField>
          </div>
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
