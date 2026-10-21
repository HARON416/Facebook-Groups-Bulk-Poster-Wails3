import { ProfileService } from '../../bindings/postika';

type AccountMethod = Exclude<keyof typeof ProfileService, 'GetProfiles' | 'CreateProfile' | 'SelectProfile'>;
type WithoutProfile<F> = F extends (profileID: string, ...args: infer A) => infer R ? (...args: A) => R : never;
type AccountService = { [K in AccountMethod]: WithoutProfile<typeof ProfileService[K]> };

// Bind once per workspace/window. Never look up a mutable global selection.
export function createAccountService(profileID: string): AccountService {
  return Object.fromEntries(Object.entries(ProfileService)
    .filter(([name]) => !['GetProfiles', 'CreateProfile', 'SelectProfile'].includes(name))
    .map(([name, method]) => [name, (...args: unknown[]) =>
      (method as (id: string, ...args: unknown[]) => unknown)(profileID, ...args)])) as AccountService;
}

export const windowProfileID = new URLSearchParams(window.location.search).get('profile') ?? 'default';
export const windowAccountName = new URLSearchParams(window.location.search).get('account') ?? 'Default profile';
export const GroupsService = createAccountService(windowProfileID);
