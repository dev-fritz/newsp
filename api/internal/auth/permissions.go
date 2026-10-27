package auth

import "github.com/dev-fritz/newsp/api/internal/models"

// Permission names an action on a resource ("resource:action").
type Permission string

// Instance-level permissions. Per-user resources (reports, ideas, preferences)
// are authorized by ownership in their services instead.
const (
	PermUsersManage     Permission = "users:manage"
	PermSourcesManage   Permission = "sources:manage"
	PermProvidersManage Permission = "providers:manage"
	PermSettingsManage  Permission = "settings:manage"
	// PermJobsViewSystem lets admins follow jobs that belong to no user
	// (collections, reindexing) and read their raw errors.
	PermJobsViewSystem Permission = "jobs:view_system"
)

var rolePermissions = map[models.Role]map[Permission]bool{
	models.RoleAdmin: {
		PermUsersManage:     true,
		PermSourcesManage:   true,
		PermProvidersManage: true,
		PermSettingsManage:  true,
		PermJobsViewSystem:  true,
	},
	models.RoleMember: {},
}

// Can reports whether id holds permission p.
func Can(id Identity, p Permission) bool {
	return rolePermissions[id.Role][p]
}

// Permissions lists what a role can do, for the frontend to adapt its UI.
func Permissions(role models.Role) []Permission {
	perms := make([]Permission, 0, len(rolePermissions[role]))
	for _, p := range []Permission{PermUsersManage, PermSourcesManage, PermProvidersManage, PermSettingsManage, PermJobsViewSystem} {
		if rolePermissions[role][p] {
			perms = append(perms, p)
		}
	}
	return perms
}
