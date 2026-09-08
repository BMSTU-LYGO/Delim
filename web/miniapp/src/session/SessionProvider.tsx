import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react';
import type { PropsWithChildren } from 'react';

import { ApiError, GatewayClient, userErrorMessage } from '../api';
import type { User } from '../api';
import { maxBridge } from '../platform/maxBridge';

const storageKey = 'delim.session.token';

export type SessionStatus =
  | 'initializing'
  | 'authenticated'
  | 'auth-error'
  | 'backend-unavailable';

interface SessionContextValue {
  client: GatewayClient;
  error?: string;
  landingGroupId?: number;
  retry(): Promise<void>;
  status: SessionStatus;
  user?: User;
}

const SessionContext = createContext<SessionContextValue | null>(null);

const getInitialToken = () => {
  const persisted = sessionStorage.getItem(storageKey);
  if (persisted) return persisted;
  return import.meta.env.DEV ? import.meta.env.VITE_DEV_SESSION_TOKEN?.trim() || null : null;
};

const unavailable = (error: unknown) =>
  error instanceof TypeError ||
  (error instanceof ApiError && [502, 503, 504].includes(error.status));

export function SessionProvider({ children }: PropsWithChildren) {
  const tokenRef = useRef<string | null>(getInitialToken());
  const requestRef = useRef(0);
  const [status, setStatus] = useState<SessionStatus>('initializing');
  const [user, setUser] = useState<User>();
  const [error, setError] = useState<string>();
  const [landingGroupId, setLandingGroupId] = useState<number>();

  const clearSession = useCallback(() => {
    tokenRef.current = null;
    sessionStorage.removeItem(storageKey);
    setUser(undefined);
    setError('Сессия завершилась. Откройте мини-приложение в MAX ещё раз.');
    setStatus('auth-error');
  }, []);

  const client = useMemo(
    () =>
      new GatewayClient({
        baseUrl:
          import.meta.env.VITE_GATEWAY_URL || (import.meta.env.DEV ? 'http://localhost:8080' : ''),
        getToken: () => tokenRef.current,
        onUnauthorized: clearSession,
      }),
    [clearSession],
  );

  const retry = useCallback(async () => {
    const requestId = ++requestRef.current;
    setError(undefined);
    setStatus('initializing');

    try {
      if (!tokenRef.current) {
        const initData = maxBridge.getInitData();
        if (!initData) {
          setError('Не удалось получить данные запуска. Откройте «Делим» внутри MAX.');
          setStatus('auth-error');
          return;
        }

        const session = await client.login(initData);
        tokenRef.current = session.token;
        sessionStorage.setItem(storageKey, session.token);
        if (session.invite?.status === 'joined') setLandingGroupId(session.invite.group_id);
      }

      const currentUser = await client.me();
      if (requestRef.current !== requestId) return;
      setUser(currentUser);
      setStatus('authenticated');
    } catch (cause) {
      if (requestRef.current !== requestId) return;
      if (unavailable(cause)) {
        setError('Сервис временно недоступен. Проверьте подключение и повторите попытку.');
        setStatus('backend-unavailable');
        return;
      }
      setError(userErrorMessage(cause, 'Не удалось войти. Откройте мини-приложение заново.'));
      setStatus('auth-error');
    }
  }, [client]);

  const value = useMemo(
    () => ({ client, error, landingGroupId, retry, status, user }),
    [client, error, landingGroupId, retry, status, user],
  );

  return <SessionContext value={value}>{children}</SessionContext>;
}

export function useSession() {
  const session = useContext(SessionContext);
  if (!session) throw new Error('useSession must be used inside SessionProvider');
  return session;
}
