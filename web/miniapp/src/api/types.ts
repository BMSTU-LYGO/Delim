export type MemberRole = 'owner' | 'admin' | 'member';
export type GroupStatus = 'active' | 'archived';
export type SplitType = 'equal' | 'fixed' | 'shares' | 'percentage' | 'item';
export type ExpenseStatus = 'pending' | 'confirmed' | 'cancelled';
export type SettlementStatus = 'pending' | 'confirmed' | 'cancelled';
export type AdjustmentType = 'refund' | 'correction';
export type ReceiptStatus = 'uploaded' | 'queued' | 'processing' | 'ready' | 'failed' | 'deleted';
export type DocumentJobStatus = 'pending' | 'processing' | 'completed' | 'failed';
export type ExportFormat = 'csv' | 'pdf' | 'xlsx';
export type ExportStatus = 'pending' | 'processing' | 'ready' | 'failed';

export interface StatusResponse {
  status: string;
}

export interface ReadyResponse {
  status: 'ok' | 'unavailable';
  core: 'ok' | 'unavailable';
  document: 'ok' | 'unavailable';
  postgres: 'ok' | 'unavailable';
}

export interface MAXLoginRequest {
  init_data: string;
}

export interface MAXLoginUser {
  id: number;
  max_user_id: number;
}

export interface MAXLoginInvite {
  status: 'invalid' | 'expired' | 'unavailable' | 'join_failed' | 'joined';
  group_id?: number;
  expires_at?: string;
}

export interface MAXLoginResponse {
  token: string;
  expires_in: number;
  user: MAXLoginUser;
  start_param?: string;
  invite?: MAXLoginInvite;
}

export interface Invite {
  start_param: string;
  deep_link?: string;
  expires_at: string;
}

export interface User {
  id: number;
  max_user_id: number;
  first_name: string;
  last_name: string;
  username: string;
}

export interface Group {
  id: number;
  name: string;
  owner_id: number;
  status: GroupStatus;
  current_user_role: MemberRole;
  created_at: string;
  updated_at: string;
}

export interface GroupList {
  groups: Group[];
  next_cursor?: number;
}

export interface GroupMember {
  group_id: number;
  user_id: number;
  role: MemberRole;
  joined_at: string;
  user?: User;
}

export interface SplitParticipant {
  user_id: number;
  value?: number;
}

export interface ExpenseItemInput {
  name: string;
  amount_minor: number;
  participant_user_ids: number[];
}

export interface ExpenseInput {
  group_id?: number;
  payer_user_id: number;
  amount_minor: number;
  currency: string;
  description: string;
  expense_date: string;
  split_type: SplitType;
  participants: SplitParticipant[];
  items: ExpenseItemInput[];
}

export interface UpdateExpenseRequest {
  version: number;
  expense: ExpenseInput;
}

export interface ExpenseItem {
  id: number;
  expense_id: number;
  name: string;
  amount_minor: number;
  position: number;
}

export interface Allocation {
  id: number;
  expense_id: number;
  expense_item_id?: number;
  user_id: number;
  amount_minor: number;
}

export interface Expense {
  id: number;
  group_id: number;
  payer_user_id: number;
  created_by: number;
  amount_minor: number;
  currency: string;
  description: string;
  expense_date: string;
  split_type: SplitType;
  status: ExpenseStatus;
  version: number;
  created_at: string;
  updated_at: string;
  items: ExpenseItem[];
  allocations: Allocation[];
}

export interface ExpenseList {
  expenses: Expense[];
  next_cursor?: number;
}

export interface Balance {
  user_id: number;
  currency: string;
  net_amount_minor: number;
}

export interface BalanceEntry {
  operation_type: string;
  operation_id: number;
  currency: string;
  amount_minor: number;
  occurred_at: string;
}

export interface BalanceBreakdown {
  balance: Balance[];
  entries: BalanceEntry[];
}

export interface SettlementPlanTransfer {
  from_user_id: number;
  to_user_id: number;
  amount_minor: number;
  currency: string;
}

export interface CreateSettlementRequest {
  sender_user_id: number;
  receiver_user_id: number;
  amount_minor: number;
  currency: string;
}

export interface Settlement {
  id: number;
  group_id: number;
  sender_user_id: number;
  receiver_user_id: number;
  amount_minor: number;
  currency: string;
  status: SettlementStatus;
  created_by: number;
  version: number;
  created_at: string;
  confirmed_at?: string;
}

export interface SettlementList {
  settlements: Settlement[];
  next_cursor?: number;
}

export interface AdjustmentAllocation {
  user_id: number;
  amount_minor: number;
}

export interface CreateAdjustmentRequest {
  type: AdjustmentType;
  amount_minor: number;
  currency: string;
  allocations: AdjustmentAllocation[];
}

export interface Adjustment {
  id: number;
  group_id: number;
  expense_id: number;
  type: AdjustmentType;
  amount_minor: number;
  currency: string;
  created_by: number;
  created_at: string;
  allocations: AdjustmentAllocation[];
}

export interface Receipt {
  id: number;
  group_id: number;
  filename: string;
  content_type: 'image/jpeg' | 'image/png' | 'image/webp';
  size_bytes: number;
  status: ReceiptStatus;
  created_at: string;
}

export interface DocumentJob {
  id: number;
  receipt_id: number;
  type: 'ocr';
  status: DocumentJobStatus;
  error_code?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}

export interface CreateReceiptResponse {
  receipt: Receipt;
  job: DocumentJob;
}

export interface OCRItem {
  name: string;
  quantity?: string;
  unit_price_minor?: number;
  amount_minor: number;
  confidence: number;
}

export interface OCRResult {
  status: ReceiptStatus;
  merchant?: string;
  date?: string;
  total_minor?: number;
  currency?: string;
  items: OCRItem[];
  confidence: number;
  qr_found: boolean;
}

export interface Export {
  id: number;
  group_id: number;
  format: ExportFormat;
  status: ExportStatus;
  filename: string;
  error_code?: string;
  created_at: string;
  finished_at?: string;
}

export interface ErrorResponse {
  error: {
    code: string;
    message: string;
  };
}

export interface PageQuery {
  limit?: number;
  cursor?: number;
}

export interface DownloadedFile {
  blob: Blob;
  contentType: string;
  filename?: string;
}
