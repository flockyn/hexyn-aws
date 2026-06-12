// Package awsx wraps the AWS SDK in small, testable clients and holds the plain
// data types those clients exchange with the rest of the application.
package awsx

import "strings"

// ParameterType is a value object representing the SSM parameter type.
type ParameterType string

const (
	ParameterTypeString       ParameterType = "String"
	ParameterTypeSecureString ParameterType = "SecureString"
)

// Parameter is a single SSM parameter.
type Parameter struct {
	Name  string
	Value string
	Type  ParameterType
}

// IsSecure reports whether the parameter is stored as a SecureString.
func (p Parameter) IsSecure() bool {
	return p.Type == ParameterTypeSecureString
}

// ParamPath identifies the /<Env>/<Repo>/ prefix a region-scoped SSM client operates on.
type ParamPath struct {
	Env  string
	Repo string
}

// Session represents an authenticated AWS session.
type Session struct {
	ARN          string
	AccountID    string
	AccountAlias string
	Profile      string
	Region       string
	CredSource   string // file path of the credentials file used
}

// DisplayName returns the most descriptive account name available.
func (s Session) DisplayName() string {
	if s.AccountAlias != "" {
		return s.AccountAlias
	}
	return s.AccountID
}

// Credentials holds temporary AWS access keys.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// Cluster is an ECS cluster.
type Cluster struct {
	Name string
}

// Service is an ECS service.
type Service struct {
	Name string
}

// TaskSecret maps an environment variable name to its SSM Parameter Store path.
type TaskSecret struct {
	EnvVarName string // the KEY used in the .env file
	SSMPath    string // the full SSM parameter ARN / path
}

// Path returns the bare SSM parameter path for the secret. ECS stores a secret's
// source in either form — a full ARN
// ("arn:aws:ssm:<region>:<account>:parameter/prod/api/KEY") or the bare path
// ("/prod/api/KEY") — so the ARN prefix is stripped when present. The result
// always begins with "/".
func (t TaskSecret) Path() string {
	const marker = ":parameter"
	if i := strings.LastIndex(t.SSMPath, marker); i != -1 {
		return t.SSMPath[i+len(marker):]
	}
	return t.SSMPath
}
