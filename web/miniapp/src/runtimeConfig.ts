declare global {
  interface Window {
    __DELIM_CONFIG__?: {
      gatewayUrl?: string;
    };
  }
}

export const gatewayUrl = () => {
  const runtimeUrl = window.__DELIM_CONFIG__?.gatewayUrl?.trim();
  if (runtimeUrl) return runtimeUrl;

  if (import.meta.env.DEV) {
    return import.meta.env.VITE_GATEWAY_URL?.trim() || 'http://localhost:8080';
  }

  return '';
};
