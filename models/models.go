package models

type Config struct {
	Project        string
	Branch         string
	BaseURL        string
	Token          string
	Repository     string
	DownloadFolder string
	Artifacts      []string
	NoTrigger      bool
	ForceTrigger   bool
	Key            string
	Value          string
}

