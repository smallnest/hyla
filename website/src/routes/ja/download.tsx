import { createFileRoute } from '@tanstack/solid-router';
import DownloadPage, { downloadHead } from '../../components/DownloadPage';

export const Route = createFileRoute('/ja/download')({
  head: () => downloadHead('ja'),
  component: () => <DownloadPage locale="ja" />,
});
