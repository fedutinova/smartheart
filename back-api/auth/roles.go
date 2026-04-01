package auth

import "sync"

// Permission constants matching the DB seeds in migrations/002_auth_system.sql.
const (
	PermECGSubmit  = "ekg:submit"
	PermJobRead    = "job:read"
	PermJobReadOwn = "job:read_own"
	PermJobReadAll = "job:read_all"
	PermAdminAll   = "admin:all"
)

// Role name constants.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

var (
	permsMu     sync.RWMutex
	roleToPerms = map[string][]string{
		RoleUser:  {PermECGSubmit, PermJobReadOwn},
		RoleAdmin: {PermECGSubmit, PermJobReadAll, PermAdminAll},
	}
)

// InitPermsFromDB replaces the default role→permissions mapping with one
// loaded from the database. Call this at application startup after the
// DB connection is established.
func InitPermsFromDB(mapping map[string][]string) {
	permsMu.Lock()
	defer permsMu.Unlock()
	roleToPerms = mapping
}

// PermsForRoles returns the union of permissions for the given role names.
func PermsForRoles(roles []string) map[string]struct{} {
	permsMu.RLock()
	defer permsMu.RUnlock()
	out := make(map[string]struct{}, 8)
	for _, r := range roles {
		if perms, ok := roleToPerms[r]; ok {
			for _, p := range perms {
				out[p] = struct{}{}
			}
		}
	}
	return out
}
