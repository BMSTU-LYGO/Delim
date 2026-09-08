import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { MaxUI } from '@maxhub/max-ui';
import '@maxhub/max-ui/dist/styles.css';

import './app/theme.css';
import './components/form/form.css';
import './components/ui/ui.css';
import './features/groups/groups.css';
import './features/groups/members.css';
import './features/expenses/expenses.css';
import './features/receipts/receipts.css';
import './features/balance/balance.css';
import './features/settlements/settlements.css';
import { maxBridge } from './platform/maxBridge';
import { SessionGate } from './session/SessionGate';
import { SessionProvider } from './session/SessionProvider';

const root = document.getElementById('root');
if (!root) {
  throw new Error('Root element is missing');
}

const environment = maxBridge.getEnvironment();
const platform = environment.platform === 'ios' ? 'ios' : 'android';

createRoot(root).render(
  <StrictMode>
    <MaxUI colorScheme={environment.colorScheme} platform={platform} resetBody>
      <SessionProvider>
        <SessionGate />
      </SessionProvider>
    </MaxUI>
  </StrictMode>,
);
