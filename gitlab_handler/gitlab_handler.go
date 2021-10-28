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
	jobsStates []string,
) (jobsInfo, error) {
	var chosenJobStates []gitlab.BuildStateValue = nil
	if len(jobsStates) > 0 {
		for _, status := range jobsStates {
			chosenJobStates = append(chosenJobStates, gitlab.BuildStateValue(status))
		}
	}

	var selectedJobs = make(jobsInfo)
	currentPage := 1
	for {
		pipelineJobs, _, err := gitlabConfig.Cli.Jobs.ListPipelineJobs(
			fmt.Sprintf(
				"%s/%s",
				gitlabConfig.Config.Project,
				gitlabConfig.Config.Repository,
			),
			pipelineId,
			&gitlab.ListJobsOptions{
				ListOptions: gitlab.ListOptions{
					Page:    currentPage,
					PerPage: gitlabConfig.Config.PerPageCount,
				},
				Scope: chosenJobStates,
			},
		)
		if err != nil {
			err = fmt.Errorf(
				"failed to fetch jobs from the pipeline %d",
				pipelineId,
			)
			return nil, err
		}
		if len(pipelineJobs) == 0 {
			break
		}

		for _, job := range pipelineJobs {
			for _, artifact := range gitlabConfig.Config.Artifacts {
				if job.Name == artifact {
					selectedJobs[job.ID] = job.Name
					break
				}
			}
		}
		currentPage++
	}
	if len(selectedJobs) == len(gitlabConfig.Config.Artifacts) {
		return selectedJobs, nil
	}
	return nil, fmt.Errorf("no suitable jobs were found")
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
		nil,
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

			if job.Status == string(gitlab.Success) {
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
	limit := gitlabConfig.Config.PipelinesLimit
	currentPage := 1
	for limit > 0 {
		pipelines, _, err := gitlabConfig.Cli.Pipelines.ListProjectPipelines(
			fmt.Sprintf(
				"%s/%s",
				gitlabConfig.Config.Project,
				gitlabConfig.Config.Repository,
			),
			&gitlab.ListProjectPipelinesOptions{
				Ref: gitlab.String(gitlabConfig.Config.Branch),
				ListOptions: gitlab.ListOptions{
					Page:    currentPage,
					PerPage: gitlabConfig.Config.PerPageCount},
			},
		)
		if err != nil {
			errChan <- err
			return
		}
		if len(pipelines) == 0 {
			errChan <- fmt.Errorf("no suitable pipelines were found")
		}

		for _, pipeline := range pipelines {
			selectedJobs, err := findNeededJobs(
				gitlabConfig,
				pipeline.ID,
				[]string{string(gitlab.Success)},
			)
			if err == nil {
				jobsInfoChan <- selectedJobs
				return
			}
		}
		limit -= len(pipelines)
		currentPage++
	}
	errChan <- fmt.Errorf("no suitable pipelines were found")
}

func GetJobsWithNeededArtifacts(gitlabConfig *GitlabConfig) (jobsInfo, error) {
	jobsInfoChan := make(chan jobsInfo)
	errChan := make(chan error)
	ctx, cancel := context.WithTimeout(gitlabConfig.Ctx, gitlabConfig.Config.Timeout)
	defer cancel()

	if gitlabConfig.Config.ForceTrigger {
		fmt.Println("Triggering new pipeline...")
		go getJobsFromTriggeredPipeline(
			gitlabConfig,
			jobsInfoChan,
			errChan,
		)
	} else {
		fmt.Println("Searching for the latest suitable pipeline...")
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
			downloadArtifact(
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
