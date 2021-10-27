package gitlab_handler

import (
	"bufio"
	"bytes"
	"ci-downloader/config"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/xanzy/go-gitlab"
)

type jobsInfo map[int]string

type GitlabConfig struct {
	Config *config.Config
	Ctx    context.Context
	Cli    *gitlab.Client
}

type artifact struct {
	content *bytes.Reader
	name    string
}

func findNeededJobs(
	gitlabConfig *GitlabConfig,
	pipelineId int,
) (jobsInfo, error) {
	pipelineJobs, _, err := gitlabConfig.Cli.Jobs.ListPipelineJobs(
		fmt.Sprintf(
			"%s/%s",
			gitlabConfig.Config.Project,
			gitlabConfig.Config.Repository,
		),
		pipelineId,
		&gitlab.ListJobsOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: gitlabConfig.Config.PagePerCount,
			},
		},
	)
	if err != nil {
		err = fmt.Errorf(
			"failed to fetch jobs from the pipeline %d",
			pipelineId,
		)
		return nil, err
	}

	var selectedJobs = make(jobsInfo)

	for _, job := range pipelineJobs {
		for _, artifact := range gitlabConfig.Config.Artifacts {
			if job.Name == artifact {
				selectedJobs[job.ID] = job.Name
			}
		}
	}

	if len(selectedJobs) < len(gitlabConfig.Config.Artifacts) {
		err = fmt.Errorf(
			"not all needed jobs were found in the pipeline %d",
			pipelineId,
		)
		return nil, err
	}

	return selectedJobs, err
}

func getJobsFromTriggeredPipeline(
	gitlabConfig *GitlabConfig,
	jobsInfoChan chan jobsInfo,
	errChan chan error,
) {
	var pipeline *gitlab.Pipeline
	var variables []*gitlab.PipelineVariable
	var err error
	if gitlabConfig.Config.Key != "" && gitlabConfig.Config.Value != "" {
		variables = append(variables, &gitlab.PipelineVariable{
			Key:   gitlabConfig.Config.Key,
			Value: gitlabConfig.Config.Value,
		})
	}

	pipeline, _, err = gitlabConfig.Cli.Pipelines.CreatePipeline(
		fmt.Sprintf(
			"%s/%s",
			gitlabConfig.Config.Project,
			gitlabConfig.Config.Repository,
		),
		&gitlab.CreatePipelineOptions{
			Ref:       gitlab.String(gitlabConfig.Config.Branch),
			Variables: variables,
		},
	)
	if err != nil {
		errChan <- err
		return
	}

	selectedJobs, err := findNeededJobs(
		gitlabConfig,
		pipeline.ID,
	)
	if err != nil {
		errChan <- err
		return
	}

	for jobID := range selectedJobs {
		for {
			job, _, err := gitlabConfig.Cli.Jobs.GetJob(
				fmt.Sprintf(
					"%s/%s",
					gitlabConfig.Config.Project,
					gitlabConfig.Config.Repository,
				),
				jobID,
			)
			if err != nil {
				errChan <- err
				return
			}

			if job.Status == "success" {
				break
			} else if job.Status == "failed" {
				err := fmt.Errorf("job has failed: %v", job.Name)
				errChan <- err
				return
			} else if job.Status == "manual" {
				err := fmt.Errorf("job is manual: %v", job.Name)
				errChan <- err
				return
			}

			time.Sleep(gitlabConfig.Config.SleepStep)
		}
	}
	jobsInfoChan <- selectedJobs
}

func getJobsFromFinishedPipeline(
	gitlabConfig *GitlabConfig,
	jobsInfoChan chan jobsInfo,
	errChan chan error,
) {
	pipelines, _, err := gitlabConfig.Cli.Pipelines.ListProjectPipelines(
		fmt.Sprintf(
			"%s/%s",
			gitlabConfig.Config.Project,
			gitlabConfig.Config.Repository,
		),
		&gitlab.ListProjectPipelinesOptions{
			Ref:         gitlab.String(gitlabConfig.Config.Branch),
			ListOptions: gitlab.ListOptions{PerPage: gitlabConfig.Config.PagePerCount},
			Status:      gitlab.BuildState("success"),
		},
	)
	if err != nil {
		errChan <- err
		return
	}

	for _, pipeline := range pipelines {
		selectedJobs, err := findNeededJobs(
			gitlabConfig,
			pipeline.ID,
		)
		if err == nil {
			jobsInfoChan <- selectedJobs
			return
		}
	}

	err = fmt.Errorf("no suitable pipeline was found")
	errChan <- err
}

func GetJobsWithNeededArtifacts(gitlabConfig *GitlabConfig) (jobsInfo, error) {
	jobsInfoChan := make(chan jobsInfo)
	errChan := make(chan error)
	ctx, cancel := context.WithTimeout(gitlabConfig.Ctx, gitlabConfig.Config.Timeout)
	defer cancel()

	if gitlabConfig.Config.ForceTrigger {
		println("Triggering new pipeline...")
		go getJobsFromTriggeredPipeline(
			gitlabConfig,
			jobsInfoChan,
			errChan,
		)
	} else {
		println("Searching for the latest suitable pipeline...")
		go getJobsFromFinishedPipeline(
			gitlabConfig,
			jobsInfoChan,
			errChan,
		)
	}
	select {
	case selectedJobs := <-jobsInfoChan:
		fmt.Println("Jobs were aquired.")
		return selectedJobs, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout expired")
	}
}

func downloadArtifact(
	artifact *artifact,
	downloadFolder string,
	errChan chan error,
) {
	file, err := os.Create(
		fmt.Sprintf(
			"%s/%s.zip",
			downloadFolder,
			artifact.name,
		),
	)
	if err != nil {
		errChan <- err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	_, err = artifact.content.WriteTo(writer)
	errChan <- err
}

func DownloadArtifacts(gitlabConfig *GitlabConfig, selectedJobs jobsInfo) error {
	errChan := make(chan error, len(selectedJobs))
	for jobID, jobName := range selectedJobs {
		go func(jobID int, jobName string) {
			content, _, err := gitlabConfig.Cli.Jobs.GetJobArtifacts(
				fmt.Sprintf(
					"%s/%s",
					gitlabConfig.Config.Project,
					gitlabConfig.Config.Repository,
				),
				jobID,
				nil,
			)
			if err != nil {
				errChan <- err
			}
			go downloadArtifact(
				&artifact{
					content: content,
					name:    jobName,
				},
				gitlabConfig.Config.DownloadFolder,
				errChan,
			)
		}(jobID, jobName)
	}
	for range selectedJobs {
		err := <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func NewClient(token string, baseURL string) (*gitlab.Client, error) {
	client, err := gitlab.NewClient(
		token,
		gitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return client, err
}
