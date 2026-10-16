import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from '@tanstack/react-router';
import { Toaster } from 'sonner';
import { router } from '@/router';
import { openDatabase } from '@/db';
import './app/main.css';

/**
 * Application entry point.
 *
 * Order matters:
 *  1. Open Dexie BEFORE rendering. This is the primary database, not a cache —
 *     rendering a screen that queries an unopened database throws an error far
 *     from its cause.
 *  2. Render the router.
 *
 * The service worker is registered by vite-plugin-pwa (injectRegister), so
 * there is no manual registration here.
 */
async function bootstrap(): Promise<void> {
  const rootElement = document.getElementById('root');
  if (!rootElement) {
    throw new Error('#root element is missing from index.html');
  }

  try {
    await openDatabase();
  } catch (error) {
    // A failed Dexie open is fatal for an offline-first app: there is no
    // degraded mode where clinical data lives only in React state. Fail loudly
    // and visibly rather than rendering a shell that cannot save anything.
    console.error('Failed to open the local database:', error);
    rootElement.innerHTML = `
      <div style="font-family:system-ui;padding:2rem;max-width:40rem;margin:0 auto">
        <h1 style="font-size:1.25rem;font-weight:600;color:#7F1D1D">
          This device&rsquo;s local database could not be opened
        </h1>
        <p style="margin-top:.75rem;color:#334155;font-size:.875rem;line-height:1.5">
          The application stores patient records locally, so it cannot run without
          a working local database. This is usually caused by private browsing,
          a full disk, or a browser configured to block site data.
        </p>
        <p style="margin-top:.75rem;color:#64748B;font-size:.8125rem">
          Ask the system administrator to check the workstation&rsquo;s browser
          storage settings, then reload.
        </p>
      </div>`;
    return;
  }

  createRoot(rootElement).render(
    <StrictMode>
      <RouterProvider router={router} />
      {/* One toaster for the whole app, positioned away from the action bar so
          it never covers the primary button. */}
      <Toaster position="top-right" richColors closeButton />
    </StrictMode>,
  );
}

void bootstrap();
