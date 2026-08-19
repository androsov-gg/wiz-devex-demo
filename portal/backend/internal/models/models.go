package models

type DemoVersion struct {
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description" yaml:"description"`
}

type DemoTemplate struct {
	ID              string            `json:"id" yaml:"id"`
	Name            string            `json:"name" yaml:"name"`
	Title           string            `json:"title,omitempty" yaml:"title,omitempty"`
	Description     string            `json:"description" yaml:"description"`
	Category        string            `json:"category,omitempty" yaml:"category,omitempty"`
	Module          string            `json:"module,omitempty" yaml:"module,omitempty"`
	Icon            string            `json:"icon" yaml:"icon"`
	ContainerImage  string            `json:"containerImage" yaml:"containerImage"`
	BuildSourcePath string            `json:"buildSourcePath" yaml:"buildSourcePath"`
	Port            int               `json:"port" yaml:"port"`
	CPU             string            `json:"cpu" yaml:"cpu"`
	Memory          string            `json:"memory" yaml:"memory"`
	DefaultEnv      map[string]string `json:"defaultEnv,omitempty" yaml:"defaultEnv,omitempty"`
	DocsURL         string            `json:"docsUrl,omitempty" yaml:"docsUrl,omitempty"`
	Versions        []DemoVersion     `json:"versions" yaml:"versions"`
	SelectedVersion string            `json:"selectedVersion,omitempty" yaml:"selectedVersion,omitempty"`
}

type InstanceStatus string

const (
	StatusBuildingImage InstanceStatus = "BUILDING_IMAGE"
	StatusProvisioning  InstanceStatus = "PROVISIONING"
	StatusRestarting    InstanceStatus = "RESTARTING"
	StatusRunning       InstanceStatus = "RUNNING"
	StatusStopping      InstanceStatus = "STOPPING"
	StatusStopped       InstanceStatus = "STOPPED"
	StatusFailed        InstanceStatus = "FAILED"
)

type DemoInstance struct {
	ID              string         `json:"id" firestore:"id"`
	TemplateID      string         `json:"templateId" firestore:"templateId"`
	TemplateName    string         `json:"templateName" firestore:"templateName"`
	Version         string         `json:"version,omitempty" firestore:"version,omitempty"`
	OwnerEmail      string         `json:"ownerEmail" firestore:"ownerEmail"`
	OwnerName       string         `json:"ownerName" firestore:"ownerName"`
	Status          InstanceStatus `json:"status" firestore:"status"`
	CreatedAt       string         `json:"createdAt" firestore:"createdAt"`
	ServiceName     string         `json:"serviceName,omitempty" firestore:"serviceName,omitempty"`
	ServiceURL      string         `json:"serviceUrl,omitempty" firestore:"serviceUrl,omitempty"`
	Region          string         `json:"region,omitempty" firestore:"region,omitempty"`
	Port            int            `json:"port" firestore:"port"`
	BuildID         string         `json:"buildId,omitempty" firestore:"buildId,omitempty"`
	ErrorMessage    string         `json:"errorMessage,omitempty" firestore:"errorMessage,omitempty"`
	CloudRunReadyAt string         `json:"cloudRunReadyAt,omitempty" firestore:"cloudRunReadyAt,omitempty"`
}

type BuildRecord struct {
	BuildID    string `json:"buildId" firestore:"buildId"`
	TemplateID string `json:"templateId" firestore:"templateId"`
	Version    string `json:"version,omitempty" firestore:"version,omitempty"`
	Status     string `json:"status" firestore:"status"`
	StartTime  string `json:"startTime" firestore:"startTime"`
	FinishTime string `json:"finishTime,omitempty" firestore:"finishTime,omitempty"`
	LogURL     string `json:"logUrl,omitempty" firestore:"logUrl,omitempty"`
}

type UserSession struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Initials string `json:"initials"`
	Domain   string `json:"domain"`
}
