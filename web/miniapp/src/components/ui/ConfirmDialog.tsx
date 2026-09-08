import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useEffect, useId, useRef } from 'react';

interface ConfirmDialogProps {
  confirmLabel?: string;
  destructive?: boolean;
  description: string;
  onCancel(): void;
  onConfirm(): void;
  open: boolean;
  title: string;
}

export function ConfirmDialog({
  confirmLabel = 'Подтвердить',
  destructive = false,
  description,
  onCancel,
  onConfirm,
  open,
  title,
}: ConfirmDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      aria-describedby={descriptionId}
      aria-labelledby={titleId}
      aria-modal="true"
      className="confirm-dialog"
      onCancel={onCancel}
      ref={dialogRef}
    >
      <Flex direction="column" gap={16}>
        <Flex direction="column" gap={8}>
          <Typography.Headline asChild variant="medium">
            <h2 id={titleId}>{title}</h2>
          </Typography.Headline>
          <Typography.Body asChild color="secondary">
            <p id={descriptionId}>{description}</p>
          </Typography.Body>
        </Flex>
        <Flex gap={8} justify="end">
          <Button autoFocus onClick={onCancel} size="small" variant="secondary">
            Отмена
          </Button>
          <Button
            onClick={() => {
              onConfirm();
              onCancel();
            }}
            size="small"
            variant={destructive ? 'destructive' : 'primary'}
          >
            {confirmLabel}
          </Button>
        </Flex>
      </Flex>
    </dialog>
  );
}
