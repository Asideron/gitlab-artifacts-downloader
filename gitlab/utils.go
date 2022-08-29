package gitlab

import "fmt"

func makeProjectId(project, repo string) string {
	return fmt.Sprintf(
		"%s/%s",
		project,
		repo,
	)
}

func isFinishedJob(state string) (bool, error) {
	switch state {
	case Success:
		return true, nil
	case Failed, Canceled, Manual, Skipped:
		return true, errNotSuccessfulJob
	case Running, Pending:
		return false, nil
	default:
		return false, errUnrecognizeJobStatus
	}
}
