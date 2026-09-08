import { Button, CellList, Container, Flex, Input, Typography } from '@maxhub/max-ui';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useParams } from 'react-router-dom';

import type { Group, GroupMember, MemberRole, User } from '../../api';
import { userErrorMessage } from '../../api';
import { FormField, FormMessage, useDirtyForm, useFormSubmit } from '../../components/form';
import { EmptyState, ErrorState, PageHeader, SkeletonList, StatusBadge, UserRow } from '../../components/ui';
import { useSession } from '../../session/SessionProvider';
import { InvitePanel } from './InvitePanel';
import { ArchiveGroupAction } from './ArchiveGroupAction';

const roleLabels: Record<MemberRole, string> = {
  owner: 'Владелец',
  admin: 'Администратор',
  member: 'Участник',
};

interface MembersData {
  group: Group;
  members: GroupMember[];
}

const userFor = (member: GroupMember): User =>
  member.user ?? {
    first_name: '',
    id: member.user_id,
    last_name: '',
    max_user_id: 0,
    username: `id${member.user_id}`,
  };

const parseUserIds = (value: string) => {
  const parts = value.trim().split(/[\s,;]+/).filter(Boolean);
  if (!parts.length) return [];
  const ids = parts.map(Number);
  if (ids.some((id) => !Number.isSafeInteger(id) || id <= 0)) return undefined;
  return [...new Set(ids)];
};

export function MembersPage() {
  const { groupId } = useParams();
  const numericGroupId = Number(groupId);
  const { client, user } = useSession();
  const [data, setData] = useState<MembersData>();
  const [error, setError] = useState<string>();
  const [roleError, setRoleError] = useState<string>();
  const [updatingRole, setUpdatingRole] = useState<number>();
  const [userIdsInput, setUserIdsInput] = useState('');
  const [idsTouched, setIdsTouched] = useState(false);
  const roleUpdateInFlight = useRef(false);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!Number.isSafeInteger(numericGroupId) || numericGroupId <= 0) {
        setError('Некорректный идентификатор группы');
        return;
      }
      setError(undefined);
      try {
        const [group, members] = await Promise.all([
          client.getGroup(numericGroupId, signal),
          client.listGroupMembers(numericGroupId, signal),
        ]);
        setData({ group, members });
      } catch (cause) {
        if (cause instanceof Error && cause.name === 'AbortError') return;
        setError(userErrorMessage(cause, 'Не удалось загрузить участников'));
      }
    },
    [client, numericGroupId],
  );

  useEffect(() => {
    const controller = new AbortController();
    setData(undefined);
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const parsedIds = useMemo(() => parseUserIds(userIdsInput), [userIdsInput]);
  const idsError = parsedIds === undefined
    ? 'Используйте только положительные числовые ID'
    : parsedIds.length === 0
      ? 'Введите хотя бы один ID'
      : undefined;

  const addMembers = useCallback(
    () => client.addGroupMembers(numericGroupId, parsedIds ?? []),
    [client, numericGroupId, parsedIds],
  );
  const applyAddedMembers = useCallback((added: GroupMember[]) => {
    setData((current) => {
      if (!current) return current;
      const byId = new Map(current.members.map((member) => [member.user_id, member]));
      added.forEach((member) => byId.set(member.user_id, member));
      return { ...current, members: [...byId.values()] };
    });
    setUserIdsInput('');
    setIdsTouched(false);
  }, []);
  const addForm = useFormSubmit({
    isValid: !idsError,
    onSubmit: addMembers,
    onSuccess: applyAddedMembers,
    successMessage: 'Участники добавлены',
  });
  useDirtyForm(Boolean(userIdsInput));

  const updateRole = async (member: GroupMember, role: Exclude<MemberRole, 'owner'>) => {
    if (roleUpdateInFlight.current) return;
    roleUpdateInFlight.current = true;
    setUpdatingRole(member.user_id);
    setRoleError(undefined);
    try {
      const updated = await client.updateMemberRole(numericGroupId, member.user_id, role);
      setData((current) =>
        current
          ? {
              ...current,
              members: current.members.map((item) =>
                item.user_id === updated.user_id ? { ...updated, user: item.user } : item,
              ),
            }
          : current,
      );
    } catch (cause) {
      setRoleError(userErrorMessage(cause, 'Не удалось изменить роль'));
    } finally {
      roleUpdateInFlight.current = false;
      setUpdatingRole(undefined);
    }
  };

  if (error && !data) return <ErrorState description={error} onRetry={() => void load()} />;
  if (!data) return <SkeletonList count={5} />;

  const { group, members } = data;
  const active = group.status === 'active';
  const canAdd = active && group.current_user_role !== 'member';
  const canManageRoles = active && group.current_user_role === 'owner';

  return (
    <div className="screen members-page">
      <PageHeader
        subtitle={`${members.length} ${members.length === 1 ? 'участник' : 'участников'}`}
        title="Участники"
      />
      <Container>
        <Flex direction="column" gap={20}>
          {members.length ? (
            <CellList className="members-page__list">
              {members.map((member) => {
                const canChange = canManageRoles && member.role !== 'owner';
                const after = canChange ? (
                  <select
                    aria-label={`Роль пользователя ${userFor(member).username}`}
                    className="role-select"
                    disabled={updatingRole !== undefined}
                    onChange={(event) =>
                      void updateRole(member, event.target.value as Exclude<MemberRole, 'owner'>)
                    }
                    value={member.role}
                  >
                    <option value="member">Участник</option>
                    <option value="admin">Администратор</option>
                  </select>
                ) : (
                  <StatusBadge tone={member.role === 'owner' ? 'themed' : 'neutral'}>
                    {roleLabels[member.role]}
                  </StatusBadge>
                );
                return (
                  <UserRow
                    after={
                      <Flex align="center" gap={8}>
                        {member.user_id === user?.id ? <StatusBadge tone="positive">Вы</StatusBadge> : null}
                        {after}
                      </Flex>
                    }
                    key={member.user_id}
                    user={userFor(member)}
                  />
                );
              })}
            </CellList>
          ) : (
            <EmptyState description="Список участников пуст." title="Нет участников" />
          )}
          <FormMessage>{roleError}</FormMessage>

          {canAdd ? <InvitePanel groupId={group.id} /> : null}

          {canAdd ? (
            <details className="members-page__manual-add">
              <summary>Добавить зарегистрированных пользователей по ID</summary>
              <form className="form-stack" onSubmit={addForm.handleSubmit}>
                <FormField
                  error={idsTouched ? idsError : undefined}
                  hint="Несколько ID можно разделить пробелом или запятой"
                  htmlFor="member-ids"
                  label="ID пользователей"
                  required
                >
                  <Input
                    aria-describedby="member-ids-message"
                    aria-invalid={idsTouched && Boolean(idsError)}
                    id="member-ids"
                    inputMode="numeric"
                    onBlur={() => setIdsTouched(true)}
                    onChange={(event) => setUserIdsInput(event.target.value)}
                    placeholder="Например, 12, 34"
                    value={userIdsInput}
                  />
                </FormField>
                <FormMessage>{addForm.error}</FormMessage>
                <FormMessage tone="success">{addForm.feedback}</FormMessage>
                <Button disabled={!addForm.canSubmit} loading={addForm.submitting} type="submit">
                  Добавить
                </Button>
              </form>
            </details>
          ) : null}

          {!active ? (
            <Typography.Body color="secondary">В архивной группе роли и состав не меняются.</Typography.Body>
          ) : null}
          <ArchiveGroupAction
            group={group}
            onArchived={(archivedGroup) =>
              setData((current) => current && { ...current, group: archivedGroup })
            }
          />
        </Flex>
      </Container>
    </div>
  );
}
