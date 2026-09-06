import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import type { Expense, Group, GroupMember } from '../../api';
import { ErrorState, SkeletonList } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';
import { ExpenseForm } from './ExpenseForm';

interface ExpenseContext {
  group: Group;
  members: GroupMember[];
}

export function CreateExpensePage() {
  const { groupId } = useParams();
  const numericGroupId = Number(groupId);
  const { client } = useSession();
  const navigate = useNavigate();
  const [context, setContext] = useState<ExpenseContext>();
  const [error, setError] = useState<string>();

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!Number.isSafeInteger(numericGroupId) || numericGroupId <= 0) {
        setError('Некорректный идентификатор группы');
        return;
      }
      setError(undefined);
      try {
        const [group, members] = await Promise.all([
          client.getGroup(numericGroupId, signal),
          client.listGroupMembers(numericGroupId, signal),
        ]);
        if (group.status === 'archived') {
          setError('В архивной группе нельзя добавлять расходы');
          return;
        }
        setContext({ group, members });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(cause instanceof Error ? cause.message : 'Не удалось подготовить форму');
      }
    },
    [client, numericGroupId],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  if (error) return <ErrorState description={error} onRetry={() => void load()} />;
  if (!context) return <SkeletonList count={5} />;

  return (
    <ExpenseForm
      group={context.group}
      members={context.members}
      onSave={(input) => client.createExpense(numericGroupId, input)}
      onSaved={(expense: Expense) =>
        navigate(routes.expense(String(expense.id)), { replace: true, state: { created: true } })
      }
    />
  );
}
