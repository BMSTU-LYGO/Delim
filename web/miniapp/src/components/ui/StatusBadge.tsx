import type { HTMLAttributes } from 'react';

type BadgeTone = 'neutral' | 'positive' | 'warning' | 'negative' | 'themed';

interface StatusBadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: BadgeTone;
}

export function StatusBadge({ children, className = '', tone = 'neutral', ...props }: StatusBadgeProps) {
  return (
    <span className={`status-badge status-badge--${tone} ${className}`.trim()} {...props}>
      {children}
    </span>
  );
}
