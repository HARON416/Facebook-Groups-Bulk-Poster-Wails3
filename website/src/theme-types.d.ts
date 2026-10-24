interface SiteThemeOption {
  id: string;
  name: string;
  bg: string;
  main: string;
  sub: string;
  text: string;
  mode: 'light' | 'dark';
}
interface Window {
  PostikaTheme: {
    options: SiteThemeOption[];
    selected: string;
    preference: string | null;
    select: (id: string) => void;
  };
}
