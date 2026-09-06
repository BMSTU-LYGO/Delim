const segment = (value: string) => encodeURIComponent(value);

export const routes = {
  groups: '/',
  newGroup: '/groups/new',
  group: (groupId: string) => `/groups/${segment(groupId)}`,
  newExpense: (groupId: string) => `/groups/${segment(groupId)}/expense/new`,
  expense: (expenseId: string) => `/expenses/${segment(expenseId)}`,
  editExpense: (expenseId: string) => `/expenses/${segment(expenseId)}/edit`,
  receipt: (receiptId: string) => `/receipts/${segment(receiptId)}`,
  balance: (groupId: string) => `/groups/${segment(groupId)}/balance`,
  members: (groupId: string) => `/groups/${segment(groupId)}/members`,
  settlements: (groupId: string) => `/groups/${segment(groupId)}/settlements`,
} as const;
