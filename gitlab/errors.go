package gitlab

import "errors"

var (
	errNoMatchingJobsFound  = errors.New("no matching jobs were found")
	errNotAllJobsFound      = errors.New("not all needed jobs ere found")
	errNotSuccessfulJob     = errors.New("not successful job")
	errUnrecognizeJobStatus = errors.New("unrecognized job status")
)
