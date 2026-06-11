package awsx

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"hexyn-aws/internal/config"
)

// LoadedCredentials carries the parsed contents of the on-disk credentials file.
type LoadedCredentials struct {
	CredsMap map[string]string // aws_access_key_id, aws_secret_access_key, region, …
	Profile  string
	Source   string // absolute path of the credentials file
}

// CredentialLoader is the read-only view AWS clients need to build AWS configs.
type CredentialLoader interface {
	Load() (LoadedCredentials, error)
}

// CredentialStore reads and writes the flat credentials file in the base directory.
type CredentialStore struct {
	cfg *config.Provider
}

// NewCredentialStore creates a CredentialStore backed by the given config provider.
func NewCredentialStore(cfg *config.Provider) *CredentialStore {
	return &CredentialStore{cfg: cfg}
}

// GetPath returns the absolute path of the credentials file.
func (s *CredentialStore) GetPath() string {
	return s.cfg.CredentialsPath()
}

// Save writes temporary AWS credentials. The profile is inferred from an
// assumed-role ARN when present.
func (s *CredentialStore) Save(creds Credentials, arn string) error {
	profileName := "default"
	if strings.Contains(arn, "assumed-role") {
		parts := strings.Split(arn, "/")
		if len(parts) > 1 {
			profileName = parts[1]
		}
	}
	content := fmt.Sprintf(
		"[%s]\naws_access_key_id=%s\naws_secret_access_key=%s\naws_session_token=%s\n",
		profileName, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken,
	)
	credPath := s.cfg.CredentialsPath()
	_ = os.MkdirAll(filepath.Dir(credPath), 0o755)
	return os.WriteFile(credPath, []byte(content), 0o600)
}

// UpdateRegion rewrites (or appends) the region= line in the credentials file.
func (s *CredentialStore) UpdateRegion(region string) error {
	credPath := s.cfg.CredentialsPath()
	data, err := os.ReadFile(credPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "region=") {
			lines[i] = "region=" + region
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, "region="+region)
	}
	return os.WriteFile(credPath, []byte(strings.Join(lines, "\n")), 0o600)
}

// Load parses the credentials file into a key-value map plus metadata.
// Returns ErrCredentialsMissing when the file does not exist.
func (s *CredentialStore) Load() (LoadedCredentials, error) {
	credPath := s.cfg.CredentialsPath()
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return LoadedCredentials{Source: credPath}, ErrCredentialsMissing
	}

	file, err := os.Open(credPath)
	if err != nil {
		return LoadedCredentials{Source: credPath}, err
	}
	defer func() { _ = file.Close() }()

	credsMap := make(map[string]string)
	profile := "default"
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profile = strings.Trim(line, "[]")
			continue
		}
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			credsMap[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return LoadedCredentials{}, fmt.Errorf("error reading credentials file: %w", err)
	}

	return LoadedCredentials{CredsMap: credsMap, Profile: profile, Source: credPath}, nil
}
