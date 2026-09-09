import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';

import { routes } from '../app/routes';
import { useSession } from './SessionProvider';

const launchTarget = (intent: { action: string; group_id?: number; entity_id?: number }): string | null => {
  if (!intent.group_id) return null;
  const group = String(intent.group_id);
  switch (intent.action) {
    case 'group':
      return routes.group(group);
    case 'new_expense':
      return routes.newExpense(group);
    case 'balance':
      return routes.balance(group);
    case 'expense':
      return intent.entity_id ? routes.expense(String(intent.entity_id)) : routes.group(group);
    case 'settlement':
      return routes.settlements(group);
    default:
      return null;
  }
};

export function SessionLanding() {
  const { clearLaunchIntent, landingGroupId, launchIntent } = useSession();
  const navigate = useNavigate();
  const applied = useRef(false);

  useEffect(() => {
    if (applied.current) return;
    // Launch intent (navigation only, no membership) takes priority and is
    // consumed once; a refresh must not re-run the navigation.
    if (launchIntent) {
      applied.current = true;
      const target = launchTarget(launchIntent);
      clearLaunchIntent();
      if (target) {
        navigate(target, { replace: true });
        return;
      }
    }
    if (landingGroupId) {
      applied.current = true;
      navigate(routes.group(String(landingGroupId)), { replace: true, state: { joined: true } });
    }
  }, [clearLaunchIntent, landingGroupId, launchIntent, navigate]);

  return null;
}