package auth

import "github.com/google/uuid"

// CanAccessResource reports whether the given claims grant access to a resource
// owned by resourceOwnerID. Admins and users with read-all permission can access
// any resource; regular users can only access their own.
func CanAccessResource(claims *Claims, resourceOwnerID uuid.UUID) bool {
	perms := PermsForRoles(claims.Roles)
	if _, ok := perms[PermAdminAll]; ok {
		return true
	}
	if _, ok := perms[PermJobReadAll]; ok {
		return true
	}
	claimUID, err := uuid.Parse(claims.UserID)
	return err == nil && claimUID == resourceOwnerID
}
