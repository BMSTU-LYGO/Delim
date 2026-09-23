import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useRef, useState } from 'react';

import { userErrorMessage, type Export, type ExportFormat } from '../../api';
import { FormField, FormMessage } from '../../components/form';
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

const formatNames: Record<ExportFormat, string> = {
  csv: 'CSV',
  pdf: 'PDF',
  xlsx: 'XLSX',
};

export function ExportPanel({ groupId }: ExportPanelProps) {
  const { client } = useSession();
  const [format, setFormat] = useState<ExportFormat>('pdf');
  const [currentExport, setCurrentExport] = useState<Export>();
  const [creating, setCreating] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [error, setError] = useState<string>();
  const [feedback, setFeedback] = useState<string>();
  const creatingRef = useRef(false);
  const sharingRef = useRef(false);

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

  const share = useCallback(async () => {
    if (!currentExport || currentExport.status !== 'ready' || sharingRef.current) return;
    sharingRef.current = true;
    setSharing(true);
    setError(undefined);
    setFeedback(undefined);
    try {
      const freshExport = await client.getExport(currentExport.id);
      setCurrentExport(freshExport);
      if (freshExport.status !== 'ready' || !freshExport.download_url) {
        throw new Error('Export is not ready for sharing');
      }
      await maxBridge.share({
        text: `Экспорт расходов из «Делим» в формате ${formatNames[freshExport.format]}`,
        url: freshExport.download_url,
      });
      setFeedback('Окно отправки файла открыто.');
    } catch (cause) {
      setError(userErrorMessage(cause, 'Не удалось открыть отправку файла в MAX. Повторите попытку.'));
    } finally {
      sharingRef.current = false;
      setSharing(false);
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
        <div className="export-panel__actions">
          <Button
            disabled={sharing}
            loading={sharing}
            onClick={() => void share()}
            size="medium"
            stretched
          >
            Поделиться {formatNames[currentExport.format]} в MAX
          </Button>
        </div>
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
