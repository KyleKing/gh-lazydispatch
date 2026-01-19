package runner

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/repository"
)

// Repository represents a GitHub repository.
type Repository struct {
	Owner string
	Name  string
}

// RepositoryDetector detects the current GitHub repository.
type RepositoryDetector interface {
	Current() (Repository, error)
}

type defaultRepositoryDetector struct{}

func (d defaultRepositoryDetector) Current() (Repository, error) {
	repo, err := repository.Current()
	if err != nil {
		return Repository{}, err
	}
	return Repository{Owner: repo.Owner, Name: repo.Name}, nil
}

var detector RepositoryDetector = defaultRepositoryDetector{}

// DetectRepo returns the current repository in "owner/repo" format.
func DetectRepo() (string, error) {
	return DetectRepoWithDetector(detector)
}

func DetectRepoWithDetector(det RepositoryDetector) (string, error) {
	repo, err := det.Current()
	if err != nil {
		return "", fmt.Errorf("failed to detect repository: %w", err)
	}

	return fmt.Sprintf("%s/%s", repo.Owner, repo.Name), nil
}
