import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';

import { AppShell } from './AppShell';
import { PlaceholderPage } from './PlaceholderPage';
import { GroupsPage } from '../features/groups/GroupsPage';
import { CreateGroupPage } from '../features/groups/CreateGroupPage';
import { GroupDashboardPage } from '../features/groups/GroupDashboardPage';
import { SessionLanding } from '../session/SessionLanding';
import { MembersPage } from '../features/groups/MembersPage';
import { CreateExpensePage } from '../features/expenses/CreateExpensePage';
import { ExpenseDetailsPage } from '../features/expenses/ExpenseDetailsPage';

export function App() {
  return (
    <BrowserRouter>
      <SessionLanding />
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<GroupsPage />} />
          <Route path="groups/new" element={<CreateGroupPage />} />
          <Route path="groups/:groupId" element={<GroupDashboardPage />} />
          <Route
            path="groups/:groupId/expense/new"
            element={<CreateExpensePage />}
          />
          <Route path="expenses/:expenseId" element={<ExpenseDetailsPage />} />
          <Route
            path="expenses/:expenseId/edit"
            element={<PlaceholderPage title="Редактирование расхода" />}
          />
          <Route path="receipts/:receiptId" element={<PlaceholderPage title="Чек" />} />
          <Route path="groups/:groupId/balance" element={<PlaceholderPage title="Баланс" />} />
          <Route
            path="groups/:groupId/members"
            element={<MembersPage />}
          />
          <Route
            path="groups/:groupId/settlements"
            element={<PlaceholderPage title="Взаиморасчёты" />}
          />
        </Route>
        <Route path="*" element={<Navigate replace to="/" />} />
      </Routes>
    </BrowserRouter>
  );
}
