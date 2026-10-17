/**
 * A human label for this workstation, sent as the device name.
 *
 * "Triage laptop 2" is what an admin should see, but the device cannot know that
 * — so it sends what it can prove: platform and browser. The admin renames it at
 * enrolment, and the label then means something physically checkable.
 *
 * Deliberately coarse. A full user-agent string is a fingerprinting aid and
 * unreadable in a table; `navigator.userAgentData` is Chromium-only and would
 * need a fallback anyway.
 */

const APP_VERSION = '0.1.0';

function platformName(): string {
  const ua = navigator.userAgent;
  if (ua.includes('Android')) return 'Android';
  if (/iPhone|iPad|iPod/.test(ua)) return 'iOS';
  if (ua.includes('Windows')) return 'Windows';
  if (ua.includes('Mac OS X') || ua.includes('Macintosh')) return 'macOS';
  if (ua.includes('Linux')) return 'Linux';
  return 'Unknown OS';
}

function browserName(): string {
  const ua = navigator.userAgent;
  // Order matters: Edge and Opera both contain "Chrome/", and every Chromium
  // browser contains "Safari/". Check the most specific token first.
  if (ua.includes('Edg/')) return 'Edge';
  if (ua.includes('OPR/')) return 'Opera';
  if (ua.includes('Firefox/')) return 'Firefox';
  if (ua.includes('Chrome/')) return 'Chrome';
  if (ua.includes('Safari/')) return 'Safari';
  return 'Browser';
}

/** e.g. "Windows · Chrome" — the default device label. */
export function describeDevice(): string {
  return `${platformName()} · ${browserName()}`;
}

/** e.g. "Windows 11 / Chrome 141" — free-form, shown in the admin list. */
export function describePlatform(): string {
  return navigator.userAgent;
}

export { APP_VERSION };
