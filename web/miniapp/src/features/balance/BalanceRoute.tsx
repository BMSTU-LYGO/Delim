import { useParams, useSearchParams } from 'react-router-dom';

import { ErrorState } from '../../components/ui';
import { BalanceBreakdownPage } from './BalanceBreakdownPage';
import { BalancePage } from './BalancePage';

export function BalanceRoute() {
  const { groupId } = useParams();
  const [search] = useSearchParams();
  const numericGroupId = Number(groupId);
  const userParam = search.get('user');

  if (userParam === null) return <BalancePage />;
  const userId = Number(userParam);
  if (
    !Number.isSafeInteger(numericGroupId) ||
    numericGroupId <= 0 ||
    !Number.isSafeInteger(userId) ||
    userId <= 0
  ) {
    return <ErrorState description="Некорректный участник или группа" />;
  }
  return <BalanceBreakdownPage groupId={numericGroupId} userId={userId} />;
}
