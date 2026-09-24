package constants

const (
	RoleScheduler = "scheduler"
	RoleReviewer  = "reviewer"
	RoleAdmin     = "admin"
)

var Roles = []string{RoleScheduler, RoleReviewer, RoleAdmin}

func IsRole(value string) bool {
	for _, role := range Roles {
		if value == role {
			return true
		}
	}
	return false
}

func CanPlan(role string) bool {
	return role == RoleScheduler || role == RoleAdmin
}

func CanReview(role string) bool {
	return role == RoleReviewer || role == RoleAdmin
}
