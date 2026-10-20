package domain

// Profile contains the authenticated user's identity and current access rights.
type Profile struct {
	User        *User
	Roles       []string
	Permissions []string
}
