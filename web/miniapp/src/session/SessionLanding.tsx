import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';

import { routes } from '../app/routes';
import { useSession } from './SessionProvider';

export function SessionLanding() {
  const { landingGroupId } = useSession();
  const navigate = useNavigate();
  const applied = useRef(false);

  useEffect(() => {
    if (!landingGroupId || applied.current) return;
    applied.current = true;
    navigate(routes.group(String(landingGroupId)), { replace: true, state: { joined: true } });
  }, [landingGroupId, navigate]);

  return null;
}
