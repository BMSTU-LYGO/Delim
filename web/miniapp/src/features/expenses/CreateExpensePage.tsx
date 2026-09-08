import { useCallback, useEffect, useState } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';

import type { Expense, Group, GroupMember } from '../../api';
import { userErrorMessage } from '../../api';
import { ErrorState, SkeletonList } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';
import { ExpenseForm, type ExpenseFormPrefill } from './ExpenseForm';

interface ExpenseContext {
  group: Group;
  members: GroupMember[];
}

export function CreateExpensePage() {
  const { groupId } = useParams();
  const numericGroupId = Number(groupId);
  const { client, user } = useSession();
  const location = useLocation();
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
        setError(userErrorMessage(cause, 'Не удалось подготовить форму'));
      }
    },
    [client, numericGroupId],
  );

  useEffect(() => {
    const controller = new AbortController();
    setContext(undefined);
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  if (error) return <ErrorState description={error} onRetry={() => void load()} />;
  if (!context) return <SkeletonList count={5} />;

  const prefill = (location.state as { prefill?: ExpenseFormPrefill } | null)?.prefill;
  const defaultPayerId = context.members.some((member) => member.user_id === user?.id)
    ? user?.id
    : context.group.owner_id;

  return (
    <ExpenseForm
      defaultPayerId={defaultPayerId}
      group={context.group}
      members={context.members}
      onSave={(input) => client.createExpense(numericGroupId, input)}
      onSaved={(expense: Expense) =>
        navigate(routes.expense(String(expense.id)), { replace: true, state: { created: true } })
      }
      prefill={prefill}
      title={prefill ? 'Расход из чека' : 'Новый расход'}
    />
  );
}
