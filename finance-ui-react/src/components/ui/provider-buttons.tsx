import { Button } from '@/components/ui/button';

export type Provider = 'google' | 'telegram';

/** Google "G" mark, official four colours. */
function GoogleIcon() {
  return (
    <svg viewBox="0 0 18 18" aria-hidden="true" focusable="false" className="size-5">
      <path fill="#EA4335" d="M9 3.48c1.69 0 2.83.73 3.48 1.34l2.54-2.48C13.46.89 11.43 0 9 0 5.48 0 2.44 2.02.96 4.96l2.91 2.26C4.6 5.05 6.62 3.48 9 3.48z" />
      <path fill="#4285F4" d="M17.64 9.2c0-.74-.06-1.28-.19-1.84H9v3.34h4.96c-.1.83-.64 2.08-1.84 2.92l2.84 2.2c1.7-1.57 2.68-3.88 2.68-6.62z" />
      <path fill="#FBBC05" d="M3.88 10.78A5.54 5.54 0 0 1 3.58 9c0-.62.11-1.22.29-1.78L.96 4.96A9.008 9.008 0 0 0 0 9c0 1.45.35 2.82.96 4.04l2.92-2.26z" />
      <path fill="#34A853" d="M9 18c2.43 0 4.47-.8 5.96-2.18l-2.84-2.2c-.76.53-1.78.9-3.12.9-2.38 0-4.4-1.57-5.12-3.74L.97 13.04C2.45 15.98 5.48 18 9 18z" />
    </svg>
  );
}

/** Telegram paper plane in its blue circle (the plane is cut out, so it shows the button background). */
function TelegramIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false" className="size-5" fill="#2AABEE">
      <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
    </svg>
  );
}

const button = 'h-11 w-full gap-3 rounded-xl bg-background text-[15px] shadow-xs hover:shadow-sm';

/** Sign-in buttons for the SSO providers, each with its brand mark. Neutral buttons keep text contrast in light and dark. */
export function ProviderButtons({ onSignIn, google, telegram }: { onSignIn: (p: Provider) => void; google: string; telegram: string }) {
  return (
    <div className="flex flex-col gap-3">
      <Button type="button" variant="outline" className={button} onClick={() => onSignIn('google')}>
        <GoogleIcon />
        {google}
      </Button>
      <Button type="button" variant="outline" className={button} onClick={() => onSignIn('telegram')}>
        <TelegramIcon />
        {telegram}
      </Button>
    </div>
  );
}
