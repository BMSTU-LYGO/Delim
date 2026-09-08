import { Button, Flex, Typography } from '@maxhub/max-ui';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { userErrorMessage } from '../../api';
import { FormMessage, useDirtyForm } from '../../components/form';
import { maxBridge } from '../../platform/maxBridge';
import { useSession } from '../../session/SessionProvider';
import { routes } from '../../app/routes';

const maxFileSize = 10 * 1024 * 1024;
const supportedTypes = new Set(['image/jpeg', 'image/png', 'image/webp']);

const formatBytes = (bytes: number) =>
  new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 1 }).format(bytes / 1024 / 1024) + ' МБ';

interface ReceiptUploadPanelProps {
  groupId: number;
}

export function ReceiptUploadPanel({ groupId }: ReceiptUploadPanelProps) {
  const { client } = useSession();
  const navigate = useNavigate();
  const cameraRef = useRef<HTMLInputElement>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const inFlight = useRef(false);
  const scanInFlight = useRef(false);
  const [file, setFile] = useState<File>();
  const [previewUrl, setPreviewUrl] = useState<string>();
  const [qrValue, setQRValue] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [error, setError] = useState<string>();
  const [feedback, setFeedback] = useState<string>();
  const bridgeAvailable = maxBridge.getEnvironment().available;

  useDirtyForm(Boolean(file));

  useEffect(() => {
    if (!file) {
      setPreviewUrl(undefined);
      return;
    }
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  const chooseFile = (nextFile?: File) => {
    setError(undefined);
    setFeedback(undefined);
    if (!nextFile) return;
    if (!supportedTypes.has(nextFile.type)) {
      setError('Поддерживаются только JPEG, PNG и WebP');
      return;
    }
    if (nextFile.size <= 0 || nextFile.size > maxFileSize) {
      setError('Файл должен быть не больше 10 МБ');
      return;
    }
    setFile(nextFile);
  };

  const scanQR = async () => {
    if (scanInFlight.current) return;
    scanInFlight.current = true;
    setScanning(true);
    setError(undefined);
    try {
      const value = await maxBridge.openCodeReader(false);
      if (value) {
        setQRValue(value);
        setFeedback('QR-код прочитан. Добавьте фото чека, чтобы проверить позиции и сумму.');
      }
    } catch {
      setError('Не удалось открыть сканер MAX');
    } finally {
      scanInFlight.current = false;
      setScanning(false);
    }
  };

  const upload = async () => {
    if (!file || inFlight.current) return;
    inFlight.current = true;
    setLoading(true);
    setError(undefined);
    try {
      const result = await client.uploadReceipt(groupId, file);
      navigate(routes.receipt(String(result.receipt.id)), {
        state: { groupId },
      });
    } catch (cause) {
      setError(userErrorMessage(cause, 'Не удалось загрузить чек'));
    } finally {
      inFlight.current = false;
      setLoading(false);
    }
  };

  return (
    <section aria-labelledby="receipt-upload-title" className="dashboard-card receipt-upload" id="receipt-upload">
      <Flex direction="column" gap={12}>
        <Flex direction="column" gap={4}>
          <Typography.Headline asChild variant="small">
            <h3 id="receipt-upload-title">Сканировать чек</h3>
          </Typography.Headline>
          <Typography.Body color="secondary" variant="small">
            Сфотографируйте чек или выберите JPEG, PNG или WebP до 10 МБ.
          </Typography.Body>
        </Flex>

        <input
          accept="image/jpeg,image/png,image/webp"
          aria-label="Сделать фото чека"
          capture="environment"
          className="visually-hidden"
          disabled={loading || scanning}
          onChange={(event) => chooseFile(event.target.files?.[0])}
          ref={cameraRef}
          type="file"
        />
        <input
          accept="image/jpeg,image/png,image/webp"
          aria-label="Выбрать изображение чека"
          className="visually-hidden"
          disabled={loading || scanning}
          onChange={(event) => chooseFile(event.target.files?.[0])}
          ref={fileRef}
          type="file"
        />

        {previewUrl && file ? (
          <div className="receipt-upload__preview">
            <img alt="Предпросмотр выбранного чека" src={previewUrl} />
            <Flex direction="column" gap={4}>
              <Typography.Body>{file.name}</Typography.Body>
              <Typography.Body color="secondary" variant="small">
                {formatBytes(file.size)}
              </Typography.Body>
            </Flex>
          </div>
        ) : null}

        <div className="receipt-upload__actions">
          <Button
            disabled={loading || scanning}
            onClick={() => cameraRef.current?.click()}
            size="small"
            type="button"
          >
            {file ? 'Сделать другое фото' : 'Камера'}
          </Button>
          <Button
            disabled={loading || scanning}
            onClick={() => fileRef.current?.click()}
            size="small"
            type="button"
            variant="secondary"
          >
            {file ? 'Выбрать другое' : 'Из галереи'}
          </Button>
          {bridgeAvailable ? (
            <Button
              disabled={loading || scanning}
              loading={scanning}
              onClick={() => void scanQR()}
              size="small"
              type="button"
              variant="ghost"
            >
              QR в MAX
            </Button>
          ) : null}
        </div>

        {qrValue ? (
          <Typography.Body className="receipt-upload__qr" color="tertiary" variant="small">
            QR: {qrValue}
          </Typography.Body>
        ) : null}
        <FormMessage>{error}</FormMessage>
        <FormMessage tone="success">{feedback}</FormMessage>
        {file ? (
          <Button
            disabled={loading || scanning}
            loading={loading}
            onClick={() => void upload()}
            size="medium"
            stretched
          >
            Распознать чек
          </Button>
        ) : null}
      </Flex>
    </section>
  );
}
