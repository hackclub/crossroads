package db

// InEmporium says whether user can see and buy from the Smugglers' Emporium:
// every reviewer and admin, and anyone an admin has let in.
func (u *User) InEmporium() bool {
	return u.Role == UserRoleReviewer || u.Role.IsAdmin() || u.EmporiumAccessAt != nil
}
