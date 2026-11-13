import { createFileRoute } from '@tanstack/solid-router';
import DownloadPage, { downloadHead } from '../../components/DownloadPage';

export const Route = createFileRoute('/zh/download')({
  head: () => downloadHead('zh'),
  component: () => <DownloadPage locale="zh" />,
});
