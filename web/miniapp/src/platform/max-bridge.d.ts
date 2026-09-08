interface MaxViewportSize {
  width: number;
  height: number;
}

interface MaxShareContent {
  link?: string;
  text?: string;
}

interface MaxBackButton {
  readonly isVisible: boolean;
  show(): void;
  hide(): void;
  onClick(handler: () => void): void;
  offClick(handler: () => void): void;
}

interface MaxWebApp {
  readonly initData: string;
  readonly initDataUnsafe: unknown;
  readonly platform?: string;
  readonly version?: string;
  readonly deviceName?: string;
  readonly colorScheme?: string;
  readonly BackButton?: MaxBackButton;
  getViewportSize?(): Promise<MaxViewportSize>;
  enableClosingConfirmation?(): void;
  disableClosingConfirmation?(): void;
  openLink?(url: string): void;
  openMaxLink?(url: string): void;
  downloadFile?(url: string, fileName: string): Promise<unknown>;
  shareContent?(content: MaxShareContent): Promise<unknown>;
  shareMaxContent?(content: MaxShareContent): Promise<unknown>;
  openCodeReader?(fileSelect?: boolean): Promise<{ value: string }>;
}

interface Window {
  WebApp?: MaxWebApp;
}
