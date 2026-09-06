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

const getWebApp = () => window.WebApp;

const fallbackColorScheme = (): BridgeEnvironment['colorScheme'] =>
  window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';

const downloadInBrowser = (url: string, fileName: string) => {
  const link = document.createElement('a');
  link.href = url;
  link.download = fileName;
  link.rel = 'noopener';
  link.click();
};

const beforeUnloadHandler = (event: BeforeUnloadEvent) => {
  event.preventDefault();
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
    if (webApp?.downloadFile) {
      void webApp.downloadFile(url, fileName);
      return;
    }
    downloadInBrowser(url, fileName);
  },

  async share(payload: SharePayload): Promise<void> {
    const webApp = getWebApp();
    if (webApp?.shareContent) {
      await webApp.shareContent({ link: payload.url, text: payload.text });
      return;
    }

    if (navigator.share) {
      await navigator.share(payload);
      return;
    }

    const value = [payload.text, payload.url].filter(Boolean).join('\n');
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(value);
      return;
    }

    window.prompt('Скопируйте ссылку', value);
  },

  openCodeReader(fileSelect = true): void {
    getWebApp()?.openCodeReader?.(fileSelect);
  },
} as const;
