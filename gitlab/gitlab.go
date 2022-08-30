package gitlab

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/xanzy/go-gitlab"
)

const (
	sleepStep        = 10 * time.Second
	pipelinesPerPage = 20
	jobsPerPage      = 20
)

type GitlabClient struct {
	*gitlab.Client
}

func NewClient(token string, baseURL string) (*GitlabClient, error) {
	client, err := gitlab.NewClient(
		token,
		gitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return &GitlabClient{client}, err
}

type PipelineInfo struct {
	ID         *int
	Project    string
	Repository string
	Branch     string
	KeyVals    map[string]string
}

func (cli *GitlabClient) TriggerPipeline(pipelineInfo *PipelineInfo) (*int, error) {
	var variables *[]*gitlab.PipelineVariableOptions
	for key, value := range pipelineInfo.KeyVals {
		*variables = append(*variables, &gitlab.PipelineVariableOptions{
			Key:   &key,
			Value: &value,
		})
	}

	pipeline, _, err := cli.Pipelines.CreatePipeline(
		makeProjectId(pipelineInfo.Project, pipelineInfo.Repository),
		&gitlab.CreatePipelineOptions{
			Ref:       gitlab.String(pipelineInfo.Branch),
			Variables: variables,
		},
	)
	if err != nil {
		return nil, err
	}
	return &pipeline.ID, nil
}

type JobsSearch struct {
	Jobs   *[]string
	States *[]string
}

type JobInfo struct {
	ID   int
	Name string
}

type FindJobsOpts struct {
	CancelUnneededJobs bool
	// ...
}

func (cli *GitlabClient) FindJobs(
	ctx context.Context,
	pipeline *PipelineInfo,
	jobsSearch *JobsSearch,
	opts *FindJobsOpts,
) ([]*JobInfo, error) {
	neededJobs := make([]*JobInfo, 0)

	chosenJobStates := make([]gitlab.BuildStateValue, 0)
	if jobsSearch.States != nil {
		for _, status := range *jobsSearch.States {
			chosenJobStates = append(chosenJobStates, gitlab.BuildStateValue(status))
		}
	}

	var wg sync.WaitGroup

	currentPage := 1
	keepSearhing := true
	for keepSearhing {
		pipelineJobs, _, err := cli.Jobs.ListPipelineJobs(
			makeProjectId(pipeline.Project, pipeline.Repository),
			*pipeline.ID,
			&gitlab.ListJobsOptions{
				ListOptions: gitlab.ListOptions{
					Page:    currentPage,
					PerPage: jobsPerPage,
				},
				Scope: &chosenJobStates,
			},
		)
		if err != nil {
			return nil, err
		}

		if len(pipelineJobs) == 0 {
			keepSearhing = false
		} else {
			if jobsSearch.Jobs == nil {
				for _, job := range pipelineJobs {
					neededJobs = append(neededJobs, &JobInfo{job.ID, job.Name})
				}
			} else {
				for _, job := range pipelineJobs {
					isNeededJob := false
					for _, neededJob := range *jobsSearch.Jobs {
						if job.Name == neededJob {
							neededJobs = append(neededJobs, &JobInfo{job.ID, job.Name})
							isNeededJob = true
							break
						}
					}
					if opts != nil {
						if !isNeededJob && opts.CancelUnneededJobs {
							wg.Add(1)
							go func() {
								defer wg.Done()
								_, _, err := cli.Jobs.CancelJob(
									makeProjectId(pipeline.Project, pipeline.Repository),
									job.ID,
								)
								if err != nil {
									fmt.Printf("Failed to cancel job %s\n", job.Name)
									return
								}
								fmt.Printf("Job %s was canceled.\n", job.Name)
							}()
						}
						// ...
					}
					if isNeededJob && len(neededJobs) == len(*jobsSearch.Jobs) {
						keepSearhing = false
					}
				}
			}
			currentPage++
		}
	}

	wg.Wait()

	if jobsSearch.Jobs != nil {
		if len(neededJobs) == 0 {
			return nil, errNoMatchingJobsFound
		}
		if len(neededJobs) != len(*jobsSearch.Jobs) {
			return neededJobs, errNotAllJobsFound
		}
	}

	return neededJobs, nil
}

type Artifact struct {
	Name    string
	Content *bytes.Reader
}

func (cli *GitlabClient) WaitJobArtifact(
	ctx context.Context,
	pipelineInfo *PipelineInfo,
	jobInfo *JobInfo,
) (*Artifact, error) {
	waitInterval := time.Duration(10) * time.Second
	ticker := time.NewTicker(waitInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			job, _, err := cli.Jobs.GetJob(
				makeProjectId(pipelineInfo.Project, pipelineInfo.Repository),
				jobInfo.ID,
			)
			if err != nil {
				return nil, err
			}
			finished, err := isFinishedJob(job.Status)
			if err != nil {
				return nil, err
			}
			if finished {
				return cli.GetArtifact(pipelineInfo, jobInfo)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (cli *GitlabClient) GetArtifact(
	pipelineInfo *PipelineInfo,
	job *JobInfo,
) (*Artifact, error) {
	content, _, err := cli.Jobs.GetJobArtifacts(
		makeProjectId(pipelineInfo.Project, pipelineInfo.Repository),
		job.ID,
	)
	if err != nil {
		return nil, err
	}
	return &Artifact{job.Name, content}, nil
}

func (cli *GitlabClient) DownloadArtifact(
	artifact *Artifact,
	folder string,
) error {
	f, err := os.Create(fmt.Sprintf("%s/%s.zip", folder, artifact.Name))
	if err != nil {
		return err
	}
	_, err = artifact.Content.WriteTo(f)
	return err
}
