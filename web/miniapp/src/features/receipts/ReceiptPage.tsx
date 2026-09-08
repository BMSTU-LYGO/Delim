import { Button, Container, Flex, Spinner, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';

import { ApiError, userErrorMessage } from '../../api';
import type { OCRResult, Receipt, ReceiptStatus } from '../../api';
import { FormMessage } from '../../components/form';
import { ConfirmDialog, ErrorState, PageHeader, StatusBadge } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';
import { OCRReview } from './OCRReview';

const statusView: Record<ReceiptStatus, { label: string; tone: 'neutral' | 'warning' | 'positive' | 'negative' }> = {
  uploaded: { label: 'Загружен', tone: 'neutral' },
  queued: { label: 'В очереди', tone: 'warning' },
  processing: { label: 'Распознаём', tone: 'warning' },
  ready: { label: 'Готов к проверке', tone: 'positive' },
  failed: { label: 'Ошибка OCR', tone: 'negative' },
  deleted: { label: 'Удалён', tone: 'neutral' },
};

const processingStatuses = new Set<ReceiptStatus>(['uploaded', 'queued', 'processing']);

const formatBytes = (bytes: number) =>
  new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 1 }).format(bytes / 1024 / 1024) + ' МБ';

export function ReceiptPage() {
  const { receiptId } = useParams();
  const numericReceiptId = Number(receiptId);
  const location = useLocation();
  const navigate = useNavigate();
  const { client } = useSession();
  const initialState = location.state as { groupId?: number; jobId?: number } | null;
  const [receipt, setReceipt] = useState<Receipt>();
  const [ocr, setOCR] = useState<OCRResult>();
  const [jobId, setJobId] = useState(initialState?.jobId);
  const [error, setError] = useState<string>();
  const [actionError, setActionError] = useState<string>();
  const [pollKey, setPollKey] = useState(0);
  const [retrying, setRetrying] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const retryRef = useRef<() => void>(() => undefined);
  const retryInFlight = useRef(false);
  const deleteInFlight = useRef(false);

  useEffect(() => {
    if (!Number.isSafeInteger(numericReceiptId) || numericReceiptId <= 0) {
      setError('Некорректный идентификатор чека');
      return;
    }

    const controller = new AbortController();
    let timeout: number | undefined;
    let attempt = 0;
    let active = true;
    setReceipt(undefined);
    setOCR(undefined);
    setJobId(initialState?.jobId);

    const poll = async () => {
      setError(undefined);
      try {
        const [metadata, result] = await Promise.all([
          client.getReceipt(numericReceiptId, controller.signal),
          client.getOCRResult(numericReceiptId, controller.signal),
        ]);
        if (!active) return;
        setReceipt(metadata);
        setOCR(result);
        if (processingStatuses.has(result.status)) {
          const delay = Math.min(1_500 * 1.6 ** attempt, 8_000);
          attempt += 1;
          timeout = window.setTimeout(() => void poll(), delay);
        }
      } catch (cause) {
        if (!active || (cause instanceof Error && cause.name === 'AbortError')) return;
        if (cause instanceof ApiError && cause.isConflict) {
          const delay = Math.min(1_500 * 1.6 ** attempt, 8_000);
          attempt += 1;
          timeout = window.setTimeout(() => void poll(), delay);
          return;
        }
        setError(userErrorMessage(cause, 'Не удалось получить статус распознавания'));
      }
    };

    retryRef.current = () => {
      if (!active) return;
      if (timeout !== undefined) window.clearTimeout(timeout);
      attempt = 0;
      void poll();
    };
    void poll();

    return () => {
      active = false;
      controller.abort();
      if (timeout !== undefined) window.clearTimeout(timeout);
    };
  }, [client, initialState?.jobId, numericReceiptId, pollKey]);

  const retryOCR = async () => {
    if (retryInFlight.current) return;
    retryInFlight.current = true;
    setRetrying(true);
    setActionError(undefined);
    try {
      const job = await client.retryOCR(numericReceiptId);
      setJobId(job.id);
      setOCR((current) => current && { ...current, status: 'queued' });
      setPollKey((value) => value + 1);
    } catch (cause) {
      setActionError(userErrorMessage(cause, 'Не удалось повторить распознавание'));
    } finally {
      retryInFlight.current = false;
      setRetrying(false);
    }
  };

  const deleteReceipt = async () => {
    if (deleteInFlight.current || !receipt) return;
    deleteInFlight.current = true;
    setDeleting(true);
    setActionError(undefined);
    try {
      await client.deleteReceipt(receipt.id);
      navigate(`${routes.group(String(receipt.group_id))}?receipt=1#receipt-upload`, { replace: true });
    } catch (cause) {
      setActionError(userErrorMessage(cause, 'Не удалось удалить чек'));
    } finally {
      deleteInFlight.current = false;
      setDeleting(false);
      setConfirmDelete(false);
    }
  };

  const retryLoad = useCallback(() => retryRef.current(), []);

  if (error && !receipt) return <ErrorState description={error} onRetry={retryLoad} />;
  if (!receipt || !ocr) {
    return (
      <Container className="receipt-processing">
        <Flex align="center" direction="column" gap={12}>
          <Spinner aria-label="Получаем чек" size={32} />
          <Typography.Body color="secondary">Получаем статус чека…</Typography.Body>
        </Flex>
      </Container>
    );
  }

  const status = statusView[ocr.status];
  const groupId = receipt.group_id || initialState?.groupId;

  return (
    <div className="screen receipt-page">
      <PageHeader
        action={<StatusBadge tone={status.tone}>{status.label}</StatusBadge>}
        subtitle={`${formatBytes(receipt.size_bytes)}${jobId ? ` · Задача #${jobId}` : ''}`}
        title={receipt.filename}
      />
      <Container>
        <Flex direction="column" gap={20}>
          {processingStatuses.has(ocr.status) ? (
            <section aria-live="polite" className="receipt-state" role="status">
              <Spinner aria-hidden="true" size={40} />
              <Typography.Headline asChild variant="medium">
                <h2>{ocr.status === 'processing' ? 'Распознаём позиции' : 'Чек ждёт обработки'}</h2>
              </Typography.Headline>
              <Typography.Body color="secondary">
                Обычно это занимает меньше минуты. Экран обновится автоматически.
              </Typography.Body>
            </section>
          ) : null}

          {ocr.status === 'ready' ? (
            <OCRReview key={receipt.id} ocr={ocr} receipt={receipt} />
          ) : null}

          {ocr.status === 'failed' ? (
            <section className="receipt-state receipt-state--error">
              <Typography.Headline asChild variant="medium">
                <h2>Не удалось распознать чек</h2>
              </Typography.Headline>
              <Typography.Body color="secondary">
                Попробуйте OCR ещё раз или загрузите более чёткое фото.
              </Typography.Body>
              <Button
                disabled={retrying || deleting}
                loading={retrying}
                onClick={() => void retryOCR()}
                size="medium"
              >
                Повторить OCR
              </Button>
              <Button
                disabled={deleting}
                onClick={() => setConfirmDelete(true)}
                size="small"
                variant="destructive"
              >
                Удалить чек
              </Button>
              {groupId ? (
                <Button asChild size="small" variant="secondary">
                  <Link to={`${routes.group(String(groupId))}?receipt=1#receipt-upload`}>
                    Загрузить другое фото
                  </Link>
                </Button>
              ) : null}
            </section>
          ) : null}

          {error ? <FormMessage>{error}</FormMessage> : null}
          <FormMessage>{actionError}</FormMessage>
        </Flex>
      </Container>
      <ConfirmDialog
        confirmLabel="Удалить"
        description="Изображение и результаты распознавания будут удалены. Финансовые операции это не затронет."
        destructive
        onCancel={() => setConfirmDelete(false)}
        onConfirm={() => void deleteReceipt()}
        open={confirmDelete}
        title="Удалить чек?"
      />
    </div>
  );
}
