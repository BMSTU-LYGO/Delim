import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';

import { AppShell } from './AppShell';
import { PlaceholderPage } from './PlaceholderPage';
import { GroupsPage } from '../features/groups/GroupsPage';
import { CreateGroupPage } from '../features/groups/CreateGroupPage';
import { GroupDashboardPage } from '../features/groups/GroupDashboardPage';

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<GroupsPage />} />
          <Route path="groups/new" element={<CreateGroupPage />} />
          <Route path="groups/:groupId" element={<GroupDashboardPage />} />
          <Route
            path="groups/:groupId/expense/new"
            element={<PlaceholderPage title="Новый расход" />}
          />
          <Route path="expenses/:expenseId" element={<PlaceholderPage title="Расход" />} />
          <Route
            path="expenses/:expenseId/edit"
            element={<PlaceholderPage title="Редактирование расхода" />}
          />
          <Route path="receipts/:receiptId" element={<PlaceholderPage title="Чек" />} />
          <Route path="groups/:groupId/balance" element={<PlaceholderPage title="Баланс" />} />
          <Route
            path="groups/:groupId/members"
            element={<PlaceholderPage title="Участники" />}
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
