import { Button, Container, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { ApiError, userErrorMessage } from '../../api';
import type { Expense, Group, GroupMember } from '../../api';
import { ErrorState, PageHeader, SkeletonList } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';
import { ExpenseForm } from './ExpenseForm';

interface EditExpenseData {
  expense: Expense;
  group: Group;
  members: GroupMember[];
}

export function EditExpensePage() {
  const { expenseId } = useParams();
  const numericExpenseId = Number(expenseId);
  const { client } = useSession();
  const navigate = useNavigate();
  const [data, setData] = useState<EditExpenseData>();
  const [error, setError] = useState<string>();
  const [conflict, setConflict] = useState(false);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!Number.isSafeInteger(numericExpenseId) || numericExpenseId <= 0) {
        setError('Некорректный идентификатор расхода');
        return;
      }
      setError(undefined);
      try {
        const expense = await client.getExpense(numericExpenseId, signal);
        const [group, members] = await Promise.all([
          client.getGroup(expense.group_id, signal),
          client.listGroupMembers(expense.group_id, signal),
        ]);
        if (expense.status !== 'pending') {
          setError('Можно редактировать только расход, который ожидает подтверждения');
          return;
        }
        if (group.status === 'archived') {
          setError('В архивной группе расходы доступны только для чтения');
          return;
        }
        setData({ expense, group, members });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(userErrorMessage(cause, 'Не удалось загрузить расход'));
      }
    },
    [client, numericExpenseId],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const reload = () => {
    setConflict(false);
    setData(undefined);
    void load();
  };

  if (conflict) {
    return (
      <div className="screen conflict-page" role="alert">
        <PageHeader title="Расход уже изменён" />
        <Container>
          <Flex direction="column" gap={16}>
            <Typography.Body color="secondary">
              Кто-то сохранил новую версию раньше вас. Ваши изменения не отправлены повторно и не
              перезаписали данные на сервере.
            </Typography.Body>
            <Button onClick={reload} size="medium">
              Загрузить актуальную версию
            </Button>
            <Button asChild size="medium" variant="secondary">
              <Link to={routes.expense(String(numericExpenseId))}>К расходу</Link>
            </Button>
          </Flex>
        </Container>
      </div>
    );
  }

  if (error) return <ErrorState description={error} onRetry={reload} />;
  if (!data) return <SkeletonList count={6} />;

  return (
    <ExpenseForm
      group={data.group}
      initialExpense={data.expense}
      members={data.members}
      onConflict={(cause) => {
        if (cause instanceof ApiError && cause.isConflict) setConflict(true);
      }}
      onSave={(input) =>
        client.updateExpense(numericExpenseId, {
          expense: { ...input, group_id: data.expense.group_id },
          version: data.expense.version,
        })
      }
      onSaved={(expense) =>
        navigate(routes.expense(String(expense.id)), { replace: true, state: { updated: true } })
      }
      submitLabel="Сохранить изменения"
      title="Изменить расход"
    />
  );
}
