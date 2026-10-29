export type NotificationKind = 'info' | 'success' | 'warning' | 'error';
export interface NotificationOptions {
  id?: string;
  kind?: NotificationKind;
  message: string;
  description?: string;
  duration?: number;
  actionLabel?: string;
  onaction?: () => void;
}
export type Notification = NotificationOptions & { id: string; kind: NotificationKind; duration: number };
export const notifications = $state<{ items: Notification[] }>({ items: [] });

export function notify(options: NotificationOptions): string {
  const id = options.id ?? crypto.randomUUID();
  const kind = ['success', 'warning', 'error'].includes(options.kind ?? '') ? options.kind! : 'info';
  const notification = { ...options, id, kind, duration: options.duration ?? (kind === 'error' || kind === 'warning' ? 0 : 6000) };
  notifications.items = [...notifications.items.filter(item => item.id !== id), notification].slice(-3);
  return id;
}
export function dismissNotification(id: string) {
  notifications.items = notifications.items.filter(item => item.id !== id);
}
