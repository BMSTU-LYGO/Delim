import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { MaxUI } from '@maxhub/max-ui';
import '@maxhub/max-ui/dist/styles.css';

import { App } from './app/App';
import './app/theme.css';
import { maxBridge } from './platform/maxBridge';

const root = document.getElementById('root');
if (!root) {
  throw new Error('Root element is missing');
}

const environment = maxBridge.getEnvironment();
const platform = environment.platform === 'ios' ? 'ios' : 'android';

createRoot(root).render(
  <StrictMode>
    <MaxUI colorScheme={environment.colorScheme} platform={platform} resetBody>
      <App />
    </MaxUI>
  </StrictMode>,
);
