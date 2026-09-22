<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  UserMultiple02Icon,
  UserGroupIcon,
  WebSecurityIcon,
  ShieldKeyIcon,
} from '@hugeicons/core-free-icons'
import { useRbac } from '../../composables/useRbac'

const { hasPermission } = useRbac()
const localNavigation = ref<InstanceType<typeof Navigation> | null>(null)

const canViewUsers = computed(() => hasPermission('users.view'))
const canViewGroups = computed(() => hasPermission('usergroups.view'))
const canViewRbac = computed(() => hasPermission('rbac.view'))

onMounted(() => {
  nextTick(() => {
    const nav = localNavigation.value
    if (!nav) return

    if (canViewUsers.value) {
      nav.addNavigationLink({
        id: 'iam-users',
        name: 'iam-users',
        title: 'Users',
        path: '/admin/iam/users',
        icon: UserMultiple02Icon,
        type: 'route',
        description: 'Create and manage user accounts',
      })
    }

    if (canViewGroups.value) {
      nav.addNavigationLink({
        id: 'iam-groups',
        name: 'iam-groups',
        title: 'User Groups',
        path: '/admin/iam/groups',
        icon: UserGroupIcon,
        type: 'route',
        description: 'Organise users into groups for role assignment',
      })
    }

    if (canViewRbac.value) {
      nav.addNavigationLink({
        id: 'iam-rbac',
        name: 'iam-rbac',
        title: 'Roles',
        path: '/admin/iam/rbac',
        icon: WebSecurityIcon,
        type: 'route',
        description: 'Manage RBAC roles and permissions',
      })
      nav.addNavigationLink({
        id: 'iam-permissions',
        name: 'iam-permissions',
        title: 'Permission Catalog',
        path: '/admin/iam/rbac/permissions',
        icon: ShieldKeyIcon,
        type: 'route',
        description: 'View all permissions and which roles grant them',
      })
    }
  })
})
</script>

<template>
  <Section
    subtitle="Manage users, groups, and role-based access control."
    :padding="false"
  >
    <template #title>
      <span class="section-title-with-icon">
        <HugeiconsIcon :icon="WebSecurityIcon" width="22" height="22" aria-hidden="true" />
        Identity &amp; Access Management
      </span>
    </template>

    <Navigation ref="localNavigation">
      <NavigationGrid />
    </Navigation>
  </Section>
</template>

<style scoped>
.section-title-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 0.45em;
  vertical-align: middle;
}
</style>
