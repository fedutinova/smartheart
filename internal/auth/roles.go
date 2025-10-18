package auth

// permissions are strings like "ekg:submit", "job:read", "admin:*"
const (
	PermEKGSubmit  = "ekg:submit"
	PermJobRead    = "job:read"
	PermJobReadOwn = "job:read_own"
	PermJobReadAll = "job:read_all"
	PermAdminAll   = "admin:*"
)

var roleToPerms = map[string][]string{
	"user":  {PermEKGSubmit, PermJobReadOwn},
	"admin": {PermEKGSubmit, PermJobReadAll, PermAdminAll},
}

func PermsForRoles(roles []string) map[string]struct{} {
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
