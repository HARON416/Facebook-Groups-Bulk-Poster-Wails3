interface Window {
  PostikaI18n: {
    t(source: string, ...args: (string | number)[]): string;
    apply(): void;
    select(locale: string | null, persist?: boolean): Promise<void>;
    readonly locale: string;
  };
}
