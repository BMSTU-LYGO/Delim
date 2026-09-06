import type {
  Adjustment,
  Balance,
  BalanceBreakdown,
  CreateAdjustmentRequest,
  CreateReceiptResponse,
  CreateSettlementRequest,
  DocumentJob,
  DownloadedFile,
  ErrorResponse,
  Expense,
  ExpenseInput,
  ExpenseList,
  Export,
  ExportFormat,
  Group,
  GroupList,
  GroupMember,
  Invite,
  MAXLoginResponse,
  MemberRole,
  OCRResult,
  PageQuery,
  ReadyResponse,
  Settlement,
  SettlementList,
  SettlementPlanTransfer,
  StatusResponse,
  UpdateExpenseRequest,
  User,
} from './types';

type ResponseType = 'json' | 'blob' | 'void';

interface RequestOptions {
  auth?: boolean;
  body?: unknown;
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  responseType?: ResponseType;
  signal?: AbortSignal;
}

export interface GatewayClientOptions {
  baseUrl?: string;
  fetch?: typeof window.fetch;
  getToken?: () => string | null;
  onUnauthorized?: () => void;
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
    public readonly requestId?: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }

  get isConflict() {
    return this.status === 409;
  }

  get isUnauthorized() {
    return this.status === 401;
  }
}

const normalizedBaseUrl = (value: string) => value.replace(/\/$/, '');
const resource = (value: number | string) => encodeURIComponent(String(value));

const queryString = (query: PageQuery = {}) => {
  const params = new URLSearchParams();
  if (query.limit !== undefined) params.set('limit', String(query.limit));
  if (query.cursor !== undefined) params.set('cursor', String(query.cursor));
  const value = params.toString();
  return value ? `?${value}` : '';
};

const parseFilename = (header: string | null) => {
  const encoded = header?.match(/filename\*=UTF-8''([^;]+)/i)?.[1];
  if (encoded) return decodeURIComponent(encoded);
  return header?.match(/filename="?([^";]+)"?/i)?.[1];
};

const parseError = async (response: Response): Promise<ApiError> => {
  let payload: ErrorResponse | undefined;
  try {
    payload = (await response.json()) as ErrorResponse;
  } catch {
    // The status and request id still make a non-JSON gateway error actionable.
  }
  return new ApiError(
    response.status,
    payload?.error.code ?? 'http_error',
    payload?.error.message ?? `Gateway returned ${response.status}`,
    response.headers.get('x-request-id') ?? undefined,
  );
};

export class GatewayClient {
  private readonly baseUrl: string;
  private readonly fetcher: typeof window.fetch;
  private readonly getToken: () => string | null;
  private readonly onUnauthorized: () => void;

  constructor(options: GatewayClientOptions = {}) {
    this.baseUrl = normalizedBaseUrl(options.baseUrl ?? import.meta.env.VITE_GATEWAY_URL ?? '');
    this.fetcher = options.fetch ?? window.fetch.bind(window);
    this.getToken = options.getToken ?? (() => null);
    this.onUnauthorized = options.onUnauthorized ?? (() => undefined);
  }

  private async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const headers = new Headers({ Accept: 'application/json' });
    const token = options.auth === false ? null : this.getToken();
    if (token) headers.set('Authorization', `Bearer ${token}`);

    let body: BodyInit | undefined;
    if (options.body instanceof FormData) {
      body = options.body;
    } else if (options.body !== undefined) {
      headers.set('Content-Type', 'application/json');
      body = JSON.stringify(options.body);
    }

    const response = await this.fetcher(`${this.baseUrl}${path}`, {
      body,
      headers,
      method: options.method ?? 'GET',
      signal: options.signal,
    });

    if (!response.ok) {
      if (response.status === 401) this.onUnauthorized();
      throw await parseError(response);
    }
    if (options.responseType === 'void' || response.status === 204) return undefined as T;
    if (options.responseType === 'blob') {
      return {
        blob: await response.blob(),
        contentType: response.headers.get('content-type') ?? 'application/octet-stream',
        filename: parseFilename(response.headers.get('content-disposition')),
      } as T;
    }
    return (await response.json()) as T;
  }

  health(signal?: AbortSignal) {
    return this.request<StatusResponse>('/health', { auth: false, signal });
  }

  ready(signal?: AbortSignal) {
    return this.request<ReadyResponse>('/health/ready', { auth: false, signal });
  }

  login(initData: string, signal?: AbortSignal) {
    return this.request<MAXLoginResponse>('/api/v1/auth/max', {
      auth: false,
      body: { init_data: initData },
      method: 'POST',
      signal,
    });
  }

  me(signal?: AbortSignal) {
    return this.request<User>('/api/v1/me', { signal });
  }

  listGroups(query?: PageQuery, signal?: AbortSignal) {
    return this.request<GroupList>(`/api/v1/groups${queryString(query)}`, { signal });
  }

  createGroup(name: string, signal?: AbortSignal) {
    return this.request<Group>('/api/v1/groups', { body: { name }, method: 'POST', signal });
  }

  getGroup(groupId: number, signal?: AbortSignal) {
    return this.request<Group>(`/api/v1/groups/${resource(groupId)}`, { signal });
  }

  joinGroup(groupId: number, signal?: AbortSignal) {
    return this.request<GroupMember>(`/api/v1/groups/${resource(groupId)}/join`, {
      method: 'POST',
      signal,
    });
  }

  listGroupMembers(groupId: number, signal?: AbortSignal) {
    return this.request<GroupMember[]>(`/api/v1/groups/${resource(groupId)}/members`, { signal });
  }

  addGroupMembers(groupId: number, userIds: number[], signal?: AbortSignal) {
    return this.request<GroupMember[]>(`/api/v1/groups/${resource(groupId)}/members`, {
      body: { user_ids: userIds },
      method: 'POST',
      signal,
    });
  }

  updateMemberRole(groupId: number, userId: number, role: MemberRole, signal?: AbortSignal) {
    return this.request<GroupMember>(
      `/api/v1/groups/${resource(groupId)}/members/${resource(userId)}/role`,
      { body: { role }, method: 'PATCH', signal },
    );
  }

  archiveGroup(groupId: number, signal?: AbortSignal) {
    return this.request<Group>(`/api/v1/groups/${resource(groupId)}/archive`, {
      method: 'POST',
      signal,
    });
  }

  listExpenses(groupId: number, query?: PageQuery, signal?: AbortSignal) {
    return this.request<ExpenseList>(
      `/api/v1/groups/${resource(groupId)}/expenses${queryString(query)}`,
      { signal },
    );
  }

  createExpense(groupId: number, expense: ExpenseInput, signal?: AbortSignal) {
    return this.request<Expense>(`/api/v1/groups/${resource(groupId)}/expenses`, {
      body: expense,
      method: 'POST',
      signal,
    });
  }

  getExpense(expenseId: number, signal?: AbortSignal) {
    return this.request<Expense>(`/api/v1/expenses/${resource(expenseId)}`, { signal });
  }

  updateExpense(expenseId: number, update: UpdateExpenseRequest, signal?: AbortSignal) {
    return this.request<Expense>(`/api/v1/expenses/${resource(expenseId)}`, {
      body: update,
      method: 'PUT',
      signal,
    });
  }

  confirmExpense(expenseId: number, signal?: AbortSignal) {
    return this.request<Expense>(`/api/v1/expenses/${resource(expenseId)}/confirm`, {
      method: 'POST',
      signal,
    });
  }

  cancelExpense(expenseId: number, signal?: AbortSignal) {
    return this.request<Expense>(`/api/v1/expenses/${resource(expenseId)}/cancel`, {
      method: 'POST',
      signal,
    });
  }

  getBalances(groupId: number, signal?: AbortSignal) {
    return this.request<Balance[]>(`/api/v1/groups/${resource(groupId)}/balance`, { signal });
  }

  getBalanceBreakdown(groupId: number, userId: number, signal?: AbortSignal) {
    return this.request<BalanceBreakdown>(
      `/api/v1/groups/${resource(groupId)}/balance/${resource(userId)}`,
      { signal },
    );
  }

  getSettlementPlan(groupId: number, signal?: AbortSignal) {
    return this.request<SettlementPlanTransfer[]>(
      `/api/v1/groups/${resource(groupId)}/settlement-plan`,
      { signal },
    );
  }

  listSettlements(groupId: number, query?: PageQuery, signal?: AbortSignal) {
    return this.request<SettlementList>(
      `/api/v1/groups/${resource(groupId)}/settlements${queryString(query)}`,
      { signal },
    );
  }

  createSettlement(groupId: number, input: CreateSettlementRequest, signal?: AbortSignal) {
    return this.request<Settlement>(`/api/v1/groups/${resource(groupId)}/settlements`, {
      body: input,
      method: 'POST',
      signal,
    });
  }

  confirmSettlement(settlementId: number, signal?: AbortSignal) {
    return this.request<Settlement>(`/api/v1/settlements/${resource(settlementId)}/confirm`, {
      method: 'POST',
      signal,
    });
  }

  listAdjustments(expenseId: number, signal?: AbortSignal) {
    return this.request<Adjustment[]>(`/api/v1/expenses/${resource(expenseId)}/adjustments`, {
      signal,
    });
  }

  createAdjustment(expenseId: number, input: CreateAdjustmentRequest, signal?: AbortSignal) {
    return this.request<Adjustment>(`/api/v1/expenses/${resource(expenseId)}/adjustments`, {
      body: input,
      method: 'POST',
      signal,
    });
  }

  createInvite(groupId: number, signal?: AbortSignal) {
    return this.request<Invite>(`/api/v1/groups/${resource(groupId)}/invite`, {
      method: 'POST',
      signal,
    });
  }

  uploadReceipt(groupId: number, file: File, signal?: AbortSignal) {
    const body = new FormData();
    body.append('file', file);
    return this.request<CreateReceiptResponse>(`/api/v1/groups/${resource(groupId)}/receipts`, {
      body,
      method: 'POST',
      signal,
    });
  }

  getReceipt(receiptId: number, signal?: AbortSignal) {
    return this.request<import('./types').Receipt>(`/api/v1/receipts/${resource(receiptId)}`, {
      signal,
    });
  }

  deleteReceipt(receiptId: number, signal?: AbortSignal) {
    return this.request<void>(`/api/v1/receipts/${resource(receiptId)}`, {
      method: 'DELETE',
      responseType: 'void',
      signal,
    });
  }

  getDocumentJob(jobId: number, signal?: AbortSignal) {
    return this.request<DocumentJob>(`/api/v1/document-jobs/${resource(jobId)}`, { signal });
  }

  getOCRResult(receiptId: number, signal?: AbortSignal) {
    return this.request<OCRResult>(`/api/v1/receipts/${resource(receiptId)}/ocr`, { signal });
  }

  retryOCR(receiptId: number, signal?: AbortSignal) {
    return this.request<DocumentJob>(`/api/v1/receipts/${resource(receiptId)}/retry`, {
      method: 'POST',
      signal,
    });
  }

  createExport(groupId: number, format: ExportFormat, signal?: AbortSignal) {
    return this.request<Export>(`/api/v1/groups/${resource(groupId)}/exports`, {
      body: { format },
      method: 'POST',
      signal,
    });
  }

  getExport(exportId: number, signal?: AbortSignal) {
    return this.request<Export>(`/api/v1/exports/${resource(exportId)}`, { signal });
  }

  downloadExport(exportId: number, signal?: AbortSignal) {
    return this.request<DownloadedFile>(`/api/v1/exports/${resource(exportId)}/download`, {
      responseType: 'blob',
      signal,
    });
  }
}
