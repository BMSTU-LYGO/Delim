import { Avatar, CellSimple } from '@maxhub/max-ui';
import type { ReactNode } from 'react';

import type { User } from '../../api';

const displayName = (user: Pick<User, 'first_name' | 'last_name' | 'username'>) =>
  [user.first_name, user.last_name].filter(Boolean).join(' ') || user.username || 'Участник';

const initials = (user: Pick<User, 'first_name' | 'last_name' | 'username'>) => {
  const source = [user.first_name, user.last_name].filter(Boolean);
  return (source.length ? source : [user.username || '?'])
    .map((value) => value.charAt(0))
    .join('')
    .slice(0, 2)
    .toLocaleUpperCase('ru-RU');
};

interface UserAvatarProps {
  size?: number;
  user: Pick<User, 'first_name' | 'last_name' | 'username'>;
}

export function UserAvatar({ size = 40, user }: UserAvatarProps) {
  return (
    <Avatar.Container aria-label={displayName(user)} form="circle" role="img" size={size}>
      <Avatar.Text gradient="blue">{initials(user)}</Avatar.Text>
    </Avatar.Container>
  );
}

interface UserRowProps {
  after?: ReactNode;
  subtitle?: ReactNode;
  user: User;
}

export function UserRow({ after, subtitle, user }: UserRowProps) {
  return (
    <CellSimple
      after={after}
      before={<UserAvatar user={user} />}
      subtitle={subtitle ?? (user.username ? `@${user.username}` : undefined)}
      title={displayName(user)}
    />
  );
}
