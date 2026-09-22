import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useRef, useState } from 'react';

import type { Export, ExportFormat } from '../../api';
import { FormField, FormMessage } from '../../components/form';
import { StatusBadge } from '../../components/ui';
import { maxBridge } from '../../platform/maxBridge';
import { useSession } from '../../session/SessionProvider';

interface ExportPanelProps {
  groupId: number;
}

const formatLabels: Record<ExportFormat, string> = {
  csv: 'CSV — для таблиц',
  pdf: 'PDF — для печати',
  xlsx: 'XLSX — для Excel',
};

const statusLabels: Record<Export['status'], string> = {
  failed: 'Не удалось подготовить',
  pending: 'В очереди',
  processing: 'Подготавливаем',
  ready: 'Готов к скачиванию',
};

const statusTone = (status: Export['status']) => {
  if (status === 'ready') return 'positive' as const;
  if (status === 'failed') return 'negative' as const;
  return 'warning' as const;
};

export function ExportPanel({ groupId }: ExportPanelProps) {
  const { client } = useSession();
  const [format, setFormat] = useState<ExportFormat>('csv');
  const [currentExport, setCurrentExport] = useState<Export>();
  const [creating, setCreating] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [error, setError] = useState<string>();
  const creatingRef = useRef(false);
  const downloadingRef = useRef(false);

  useEffect(() => {
    if (!currentExport || !['pending', 'processing'].includes(currentExport.status)) return;

    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      try {
        const nextExport = await client.getExport(currentExport.id, controller.signal);
        setCurrentExport(nextExport);
        setError(undefined);
        if (nextExport.status === 'failed') {
          setError('Не удалось подготовить файл. Запустите экспорт ещё раз.');
        }
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError('Не удалось проверить состояние экспорта. Повторите попытку.');
        setCurrentExport((value) => (value ? { ...value } : value));
      }
    }, 1_500);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [client, currentExport]);

  const createExport = useCallback(async () => {
    if (creatingRef.current) return;
    creatingRef.current = true;
    setCreating(true);
    setError(undefined);
    try {
      setCurrentExport(await client.createExport(groupId, format));
    } catch {
      setError('Не удалось запустить экспорт. Проверьте подключение и повторите попытку.');
    } finally {
      creatingRef.current = false;
      setCreating(false);
    }
  }, [client, format, groupId]);

  const download = useCallback(async () => {
    if (!currentExport || currentExport.status !== 'ready' || downloadingRef.current) return;
    if (!currentExport.download_url) {
      setError('Ссылка на скачивание недоступна. Обновите экспорт и повторите попытку.');
      return;
    }
    downloadingRef.current = true;
    setDownloading(true);
    setError(undefined);

    // This call intentionally precedes every await: MAX requires downloadFile to
    // be invoked within the original user click gesture.
    const downloadRequest = maxBridge.downloadFile(
      currentExport.download_url,
      currentExport.filename,
    );
    try {
      await downloadRequest;
    } catch (cause) {
      const code =
        typeof cause === 'object' && cause && 'code' in cause && typeof cause.code === 'string'
          ? cause.code
          : undefined;
      if (code === 'client.download_file.invalid_params') {
        setError('MAX не принял ссылку на скачивание. Обновите экспорт и повторите попытку.');
      } else if (code === 'client.download_file.request_timeout') {
        setError('MAX не успел начать скачивание. Повторите попытку.');
      } else {
        setError('Не удалось скачать файл. Попробуйте ещё раз.');
      }
    } finally {
      downloadingRef.current = false;
      setDownloading(false);
    }
  }, [currentExport]);

  return (
    <section aria-labelledby="export-title" className="export-panel">
      <Flex align="center" className="export-panel__header" gap={12} justify="space-between">
        <div className="export-panel__heading">
          <Typography.Headline asChild variant="small">
            <h3 id="export-title">Экспорт группы</h3>
          </Typography.Headline>
          <Typography.Body color="secondary" variant="small">
            Расходы, возвраты и погашения одним файлом
          </Typography.Body>
        </div>
        {currentExport ? (
          <StatusBadge aria-live="polite" tone={statusTone(currentExport.status)}>
            {statusLabels[currentExport.status]}
          </StatusBadge>
        ) : null}
      </Flex>

      <FormField htmlFor="export-format" label="Формат">
        <select
          className="native-select"
          disabled={
            creating ||
            currentExport?.status === 'pending' ||
            currentExport?.status === 'processing'
          }
          id="export-format"
          onChange={(event) => {
            setFormat(event.target.value as ExportFormat);
            setCurrentExport(undefined);
            setError(undefined);
          }}
          value={format}
        >
          {(Object.keys(formatLabels) as ExportFormat[]).map((value) => (
            <option key={value} value={value}>
              {formatLabels[value]}
            </option>
          ))}
        </select>
      </FormField>

      {currentExport ? (
        <ol aria-label="Этапы экспорта" className="export-progress">
          <li className="export-progress__step export-progress__step--complete">Запрошен</li>
          <li
            className={`export-progress__step${currentExport.status !== 'pending' ? ' export-progress__step--complete' : ''}`}
          >
            Подготовка
          </li>
          <li
            className={`export-progress__step${currentExport.status === 'ready' ? ' export-progress__step--complete' : ''}`}
          >
            Готов
          </li>
        </ol>
      ) : null}

      <FormMessage>{error}</FormMessage>
      {currentExport?.status === 'ready' ? (
        <Button
          disabled={downloading}
          loading={downloading}
          onClick={() => void download()}
          size="medium"
          stretched
        >
          Скачать {currentExport.format.toUpperCase()}
        </Button>
      ) : (
        <Button
          disabled={Boolean(currentExport && currentExport.status !== 'failed')}
          loading={creating}
          onClick={() => void createExport()}
          size="medium"
          stretched
          variant="secondary"
        >
          {currentExport?.status === 'failed' ? 'Повторить экспорт' : 'Подготовить файл'}
        </Button>
      )}
    </section>
  );
}
