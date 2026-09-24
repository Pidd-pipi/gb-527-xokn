package constants

const (
	WindowStatusCandidate = "candidate"
	WindowStatusSubmitted = "submitted"
	WindowStatusLocked    = "locked"
	WindowStatusAllocated = "allocated"
	WindowStatusCancelled = "cancelled"
)

var WindowStatuses = []string{
	WindowStatusCandidate,
	WindowStatusSubmitted,
	WindowStatusLocked,
	WindowStatusAllocated,
	WindowStatusCancelled,
}

func IsWindowStatus(value string) bool {
	for _, candidate := range WindowStatuses {
		if value == candidate {
			return true
		}
	}
	return false
}

func CanTransitionWindow(from, to string) bool {
	allowed := map[string]map[string]bool{
		WindowStatusCandidate: {WindowStatusSubmitted: true, WindowStatusLocked: true, WindowStatusCancelled: true},
		WindowStatusSubmitted: {WindowStatusLocked: true, WindowStatusCancelled: true},
		WindowStatusLocked:    {WindowStatusAllocated: true, WindowStatusCancelled: true},
		WindowStatusAllocated: {WindowStatusCancelled: true},
	}
	return allowed[from][to]
}
