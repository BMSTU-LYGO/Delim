import { useEffect } from 'react';

import { maxBridge } from '../../platform/maxBridge';

export function useDirtyForm(isDirty: boolean) {
  useEffect(() => {
    maxBridge.setClosingConfirmation(isDirty);
    return () => maxBridge.setClosingConfirmation(false);
  }, [isDirty]);
}
