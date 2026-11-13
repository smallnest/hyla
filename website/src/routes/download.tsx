import { createFileRoute } from '@tanstack/solid-router';
import DownloadPage, { downloadHead } from '../components/DownloadPage';

export const Route = createFileRoute('/download')({
  head: () => downloadHead('en'),
  component: () => <DownloadPage locale="en" />,
});
