import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useRef, useState } from 'react';

import type { Export, ExportFormat } from '../../api';
import { FormField, FormMessage } from '../../components/form';
import { useSession } from '../../session/SessionProvider';

interface ExportPanelProps {
  groupId: number;
}

const formatLabels: Record<ExportFormat, string> = {
  csv: 'CSV — для таблиц',
  pdf: 'PDF — для печати',
  xlsx: 'XLSX — для Excel',
};

export function ExportPanel({ groupId }: ExportPanelProps) {
  const { client } = useSession();
  const [format, setFormat] = useState<ExportFormat>('pdf');
  const [currentExport, setCurrentExport] = useState<Export>();
  const [creating, setCreating] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string>();
  const [feedback, setFeedback] = useState<string>();
  const creatingRef = useRef(false);
  const sendingRef = useRef(false);

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
    setFeedback(undefined);
    try {
      setCurrentExport(await client.createExport(groupId, format));
    } catch {
      setError('Не удалось запустить экспорт. Проверьте подключение и повторите попытку.');
    } finally {
      creatingRef.current = false;
      setCreating(false);
    }
  }, [client, format, groupId]);

  const send = useCallback(async () => {
    if (!currentExport || currentExport.status !== 'ready' || sendingRef.current) return;
    sendingRef.current = true;
    setSending(true);
    setError(undefined);
    setFeedback(undefined);
    try {
      const result = await client.sendExport(currentExport.id);
      if (!result.delivered) throw new Error('Export delivery was not confirmed');
      setFeedback('Файл отправлен в личный чат с ботом MAX.');
    } catch {
      setError('Не удалось отправить файл в MAX. Подключите личные уведомления у бота и повторите попытку.');
    } finally {
      sendingRef.current = false;
      setSending(false);
    }
  }, [client, currentExport]);

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

      <FormMessage>{error}</FormMessage>
      <FormMessage tone="success">
        {feedback ?? (currentExport?.status === 'ready' ? 'Файл готов к отправке.' : undefined)}
      </FormMessage>
      {currentExport?.status === 'ready' ? (
        <Button
          disabled={sending}
          loading={sending}
          onClick={() => void send()}
          size="medium"
          stretched
        >
          Отправить файл в MAX
        </Button>
      ) : (
        <Button
          disabled={Boolean(currentExport && currentExport.status !== 'failed')}
          loading={creating || currentExport?.status === 'pending' || currentExport?.status === 'processing'}
          onClick={() => void createExport()}
          size="medium"
          stretched
          variant="secondary"
        >
          {currentExport?.status === 'failed'
            ? 'Повторить экспорт'
            : currentExport?.status === 'pending' || currentExport?.status === 'processing'
              ? 'Подготавливаем файл…'
              : 'Подготовить файл'}
        </Button>
      )}
    </section>
  );
}
