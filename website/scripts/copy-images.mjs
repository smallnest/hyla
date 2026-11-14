// The build's prerender fetches each page's images through the app and writes
// them back as text, which turns a JPEG's bytes into replacement characters.
// The originals are still in public/img, so put them back once the build ends.
import { cpSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const from = fileURLToPath(new URL('../../docs/images', import.meta.url));
const to = fileURLToPath(new URL('../dist/client/img/docs', import.meta.url));

if (!existsSync(from)) process.exit(0);
cpSync(from, to, { recursive: true });
console.log('images: docs/images -> dist/client/img/docs');
