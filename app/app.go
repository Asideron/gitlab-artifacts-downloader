package app

import (
	"context"

	"github.com/Asideron/gitlab-artifacts-downloader/gitlab"
)

type App struct {
	Ctx       context.Context
	Config    *Config
	GitlabCli *gitlab.GitlabClient
}

func NewApp(ctx context.Context) (*App, error) {
	config, err := parseFlags()
	if err != nil {
		return nil, err
	}
	gitlabCli, err := gitlab.NewClient(config.Token, config.BaseURL)
	if err != nil {
		return nil, err
	}
	return &App{
		Ctx:       ctx,
		Config:    config,
		GitlabCli: gitlabCli,
	}, nil
}
