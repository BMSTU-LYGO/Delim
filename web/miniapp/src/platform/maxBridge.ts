export interface BridgeEnvironment {
  available: boolean;
  colorScheme: 'light' | 'dark';
  deviceName?: string;
  platform: 'ios' | 'android' | 'desktop' | 'web' | 'unknown';
  version?: string;
}

export interface ViewportSize {
  height: number;
  width: number;
}

export interface SharePayload {
  text?: string;
  url?: string;
}

export type QRScanResult =
  | { status: 'success'; value: string }
  | { status: 'cancelled' }
  | { status: 'unsupported' }
  | { status: 'permission_denied' }
  | { status: 'error' };

const normalizePlatform = (platform?: string): BridgeEnvironment['platform'] => {
  switch (platform?.toLowerCase()) {
    case 'ios':
    case 'android':
    case 'desktop':
    case 'web':
      return platform.toLowerCase() as BridgeEnvironment['platform'];
    default:
      return 'unknown';
  }
};

const getWebApp = () => {
  const webApp = window.WebApp;
  return webApp?.initData ? webApp : undefined;
};

const fallbackColorScheme = (): BridgeEnvironment['colorScheme'] =>
  window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';

const downloadInBrowser = (url: string, fileName: string) => {
  const link = document.createElement('a');
  link.href = url;
  link.download = fileName;
  link.rel = 'noopener';
  link.hidden = true;
  document.body.append(link);
  link.click();
  link.remove();
};

const beforeUnloadHandler = (event: BeforeUnloadEvent) => {
  event.preventDefault();
};

const scanErrorStatus = (
  cause: unknown,
): Extract<QRScanResult, { status: 'cancelled' | 'unsupported' | 'permission_denied' | 'error' }> => {
  const error = cause as { code?: unknown; message?: unknown; name?: unknown } | undefined;
  const details = [error?.code, error?.name, error?.message]
    .filter((value): value is string => typeof value === 'string')
    .join(' ')
    .toLowerCase();

  if (/(cancel|abort)/.test(details)) return { status: 'cancelled' };
  if (/(permission|notallowed|denied|access)/.test(details)) return { status: 'permission_denied' };
  if (/(unsupported|notsupported|not supported|unavailable|notimplemented)/.test(details)) {
    return { status: 'unsupported' };
  }
  return { status: 'error' };
};

export const maxBridge = {
  getEnvironment(): BridgeEnvironment {
    const webApp = getWebApp();
    const platform = normalizePlatform(webApp?.platform);
    const colorScheme = webApp?.colorScheme === 'dark' ? 'dark' : fallbackColorScheme();

    return {
      available: Boolean(webApp),
      colorScheme,
      deviceName: webApp?.deviceName,
      platform,
      version: webApp?.version,
    };
  },

  getInitData(): string {
    return getWebApp()?.initData ?? '';
  },

  async getViewportSize(): Promise<ViewportSize> {
    return (await getWebApp()?.getViewportSize?.()) ?? {
      height: window.innerHeight,
      width: window.innerWidth,
    };
  },

  setClosingConfirmation(enabled: boolean): void {
    const webApp = getWebApp();
    if (enabled) {
      if (webApp?.enableClosingConfirmation) {
        webApp.enableClosingConfirmation();
      } else {
        window.addEventListener('beforeunload', beforeUnloadHandler);
      }
      return;
    }

    webApp?.disableClosingConfirmation?.();
    window.removeEventListener('beforeunload', beforeUnloadHandler);
  },

  onBack(handler: () => void): () => void {
    const backButton = getWebApp()?.BackButton;
    if (!backButton) {
      return () => undefined;
    }

    backButton.onClick(handler);
    backButton.show();
    return () => {
      backButton.offClick(handler);
      backButton.hide();
    };
  },

  hideBackButton(): void {
    getWebApp()?.BackButton?.hide();
  },

  openLink(url: string): void {
    const webApp = getWebApp();
    if (webApp?.openLink) {
      webApp.openLink(url);
      return;
    }
    window.open(url, '_blank', 'noopener,noreferrer');
  },

  openMaxLink(url: string): void {
    const webApp = getWebApp();
    if (webApp?.openMaxLink) {
      webApp.openMaxLink(url);
      return;
    }
    window.location.assign(url);
  },

  downloadFile(url: string, fileName: string): void {
    const webApp = getWebApp();
    if (webApp?.downloadFile && /^https:\/\//i.test(url)) {
      void webApp.downloadFile(url, fileName);
      return;
    }
    downloadInBrowser(url, fileName);
  },

  async share(payload: SharePayload): Promise<void> {
    const webApp = getWebApp();
    if (webApp?.shareMaxContent) {
      await webApp.shareMaxContent({ link: payload.url, text: payload.text });
      return;
    }
    if (webApp?.shareContent) {
      await webApp.shareContent({ link: payload.url, text: payload.text });
      return;
    }
    throw new Error('MAX share API недоступен');
  },

  async copyText(value: string): Promise<void> {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(value);
      return;
    }
    throw new Error('Clipboard API недоступен');
  },

  async scanQRCode(fileSelect = false): Promise<QRScanResult> {
    const webApp = getWebApp();
    const openCodeReader = webApp?.openCodeReader;
    if (!openCodeReader) return { status: 'unsupported' };

    try {
      // The documented MAX Bridge contract resolves to a string, rather than
      // the { value } wrapper used by the old adapter.
      const value = await openCodeReader.call(webApp, fileSelect);
      const normalized = value.trim();
      return normalized ? { status: 'success', value: normalized } : { status: 'cancelled' };
    } catch (cause) {
      return scanErrorStatus(cause);
    }
  },
} as const;
