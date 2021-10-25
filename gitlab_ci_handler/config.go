package gitlab_ci_handler

import "time"

const (
	perPageCount = 100
	timeout      = 1800 * time.Second
	sleepStep    = 10 * time.Second
)
