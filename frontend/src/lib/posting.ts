import { GroupsService } from './accounts';
import type { PostingStatus } from '../../bindings/postika/models';
export type { PostingOptions, AppSettings } from '../../bindings/postika/models';

export type Activity = PostingStatus;
export const getSettings = GroupsService.GetSettings;
export const saveSettings = GroupsService.SaveSettings;
export const openSettings = GroupsService.OpenSettings;
export const publish = GroupsService.PublishRichPostWithOptions;
export const formatDuration = (seconds: number) => {
  seconds = Math.max(0, Math.floor(seconds || 0));
  return `${Math.floor(seconds / 60).toString().padStart(2, '0')}:${(seconds % 60).toString().padStart(2, '0')}`;
};
