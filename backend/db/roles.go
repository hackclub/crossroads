package db

// IsAdmin says whether role can do everything an admin can. Superadmins are
// admins who can also read the audit log.
func (r UserRole) IsAdmin() bool {
	return r == UserRoleAdmin || r == UserRoleSuperadmin
}
