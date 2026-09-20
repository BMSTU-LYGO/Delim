import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react';
import type { PropsWithChildren } from 'react';

import { ApiError, GatewayClient, userErrorMessage } from '../api';
import type { MAXLaunch, User } from '../api';
import { maxBridge } from '../platform/maxBridge';
import { gatewayUrl } from '../runtimeConfig';

const storageKey = 'delim.session.token';

export type SessionStatus =
  | 'initializing'
  | 'authenticated'
  | 'auth-error'
  | 'backend-unavailable';

interface SessionContextValue {
  client: GatewayClient;
  clearLaunchIntent(): void;
  error?: string;
  landingGroupId?: number;
  launchIntent?: MAXLaunch;
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
  const [launchIntent, setLaunchIntent] = useState<MAXLaunch>();

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
        baseUrl: gatewayUrl(),
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
      // MAX can relaunch the Mini App with a different startapp value while a
      // previous Delim session is still stored. Re-authenticate whenever MAX
      // supplies initData so that the server sees that current launch.
      const initData = maxBridge.getInitData();
      if (initData) {
        setLandingGroupId(undefined);
        setLaunchIntent(undefined);
        const session = await client.login(initData);
        tokenRef.current = session.token;
        sessionStorage.setItem(storageKey, session.token);
        if (session.invite?.status === 'joined') {
          setLandingGroupId(session.invite.group_id);
        } else if (session.invite?.status === 'expired') {
          setError('Срок действия приглашения истёк. Попросите отправить новую ссылку.');
          setStatus('auth-error');
          return;
        } else if (session.invite?.status === 'invalid') {
          setError('Приглашение недействительно. Попросите отправить новую ссылку.');
          setStatus('auth-error');
          return;
        } else if (session.invite?.status === 'join_failed') {
          setError('Не удалось вступить в группу по приглашению. Повторите попытку.');
          setStatus('auth-error');
          return;
        } else if (session.invite?.status === 'unavailable') {
          setError('Приглашения временно недоступны. Повторите попытку позже.');
          setStatus('auth-error');
          return;
        }
        if (session.launch) setLaunchIntent(session.launch);
      } else if (!tokenRef.current) {
        setError('Не удалось получить данные запуска. Откройте «Делим» внутри MAX.');
        setStatus('auth-error');
        return;
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
    () => ({
      client,
      clearLaunchIntent: () => setLaunchIntent(undefined),
      error,
      landingGroupId,
      launchIntent,
      retry,
      status,
      user,
    }),
    [client, error, landingGroupId, launchIntent, retry, status, user],
  );

  return <SessionContext value={value}>{children}</SessionContext>;
}

export function useSession() {
  const session = useContext(SessionContext);
  if (!session) throw new Error('useSession must be used inside SessionProvider');
  return session;
}
