package events_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/hootsuite/atlantis/server/events"
	lockmocks "github.com/hootsuite/atlantis/server/events/locking/mocks"
	"github.com/hootsuite/atlantis/server/events/mocks"
	"github.com/hootsuite/atlantis/server/events/models"
	"github.com/hootsuite/atlantis/server/events/models/fixtures"
	"github.com/hootsuite/atlantis/server/events/vcs"
	vcsmocks "github.com/hootsuite/atlantis/server/events/vcs/mocks"
	. "github.com/hootsuite/atlantis/testing"
	. "github.com/petergtz/pegomock"
)

func TestCleanUpPullWorkspaceErr(t *testing.T) {
	t.Log("when workspace.Delete returns an error, we return it")
	RegisterMockTestingT(t)
	w := mocks.NewMockWorkspace()
	pce := events.PullClosedExecutor{
		Workspace: w,
	}
	err := errors.New("err")
	When(w.Delete(fixtures.Repo, fixtures.Pull)).ThenReturn(err)
	actualErr := pce.CleanUpPull(fixtures.Repo, fixtures.Pull, vcs.Github)
	Equals(t, "cleaning workspace: err", actualErr.Error())
}

func TestCleanUpPullUnlockErr(t *testing.T) {
	t.Log("when locker.UnlockByPull returns an error, we return it")
	RegisterMockTestingT(t)
	w := mocks.NewMockWorkspace()
	l := lockmocks.NewMockLocker()
	pce := events.PullClosedExecutor{
		Locker:    l,
		Workspace: w,
	}
	err := errors.New("err")
	When(l.UnlockByPull(fixtures.Repo.FullName, fixtures.Pull.Num)).ThenReturn(nil, err)
	actualErr := pce.CleanUpPull(fixtures.Repo, fixtures.Pull, vcs.Github)
	Equals(t, "cleaning up locks: err", actualErr.Error())
}

func TestCleanUpPullNoLocks(t *testing.T) {
	t.Log("when there are no locks to clean up, we don't comment")
	RegisterMockTestingT(t)
	w := mocks.NewMockWorkspace()
	l := lockmocks.NewMockLocker()
	cp := vcsmocks.NewMockClientProxy()
	pce := events.PullClosedExecutor{
		Locker:    l,
		VCSClient: cp,
		Workspace: w,
	}
	When(l.UnlockByPull(fixtures.Repo.FullName, fixtures.Pull.Num)).ThenReturn(nil, nil)
	err := pce.CleanUpPull(fixtures.Repo, fixtures.Pull, vcs.Github)
	Ok(t, err)
	cp.VerifyWasCalled(Never()).CreateComment(AnyRepo(), AnyPullRequest(), AnyString(), AnyVCSHost())
}

func TestCleanUpPullComments(t *testing.T) {
	t.Log("should comment correctly")
	RegisterMockTestingT(t)
	cases := []struct {
		Description string
		Locks       []models.ProjectLock
		Exp         string
	}{
		{
			"single lock, empty path",
			[]models.ProjectLock{
				{
					Project: models.NewProject("owner/repo", ""),
					Env:     "default",
				},
			},
			"- path: `owner/repo/.` environment: `default`",
		},
		{
			"single lock, non-empty path",
			[]models.ProjectLock{
				{
					Project: models.NewProject("owner/repo", "path"),
					Env:     "default",
				},
			},
			"- path: `owner/repo/path` environment: `default`",
		},
		{
			"single path, multiple environments",
			[]models.ProjectLock{
				{
					Project: models.NewProject("owner/repo", "path"),
					Env:     "env1",
				},
				{
					Project: models.NewProject("owner/repo", "path"),
					Env:     "env2",
				},
			},
			"- path: `owner/repo/path` environments: `env1`, `env2`",
		},
		{
			"multiple paths, multiple environments",
			[]models.ProjectLock{
				{
					Project: models.NewProject("owner/repo", "path"),
					Env:     "env1",
				},
				{
					Project: models.NewProject("owner/repo", "path"),
					Env:     "env2",
				},
				{
					Project: models.NewProject("owner/repo", "path2"),
					Env:     "env1",
				},
				{
					Project: models.NewProject("owner/repo", "path2"),
					Env:     "env2",
				},
			},
			"- path: `owner/repo/path` environments: `env1`, `env2`\n- path: `owner/repo/path2` environments: `env1`, `env2`",
		},
	}
	for _, c := range cases {
		w := mocks.NewMockWorkspace()
		cp := vcsmocks.NewMockClientProxy()
		l := lockmocks.NewMockLocker()
		pce := events.PullClosedExecutor{
			Locker:    l,
			VCSClient: cp,
			Workspace: w,
		}
		t.Log("testing: " + c.Description)
		When(l.UnlockByPull(fixtures.Repo.FullName, fixtures.Pull.Num)).ThenReturn(c.Locks, nil)
		err := pce.CleanUpPull(fixtures.Repo, fixtures.Pull, vcs.Github)
		Ok(t, err)
		_, _, comment, _ := cp.VerifyWasCalledOnce().CreateComment(AnyRepo(), AnyPullRequest(), AnyString(), AnyVCSHost()).GetCapturedArguments()

		expected := "Locks and plans deleted for the projects and environments modified in this pull request:\n\n" + c.Exp
		Equals(t, expected, comment)
	}
}

func AnyRepo() models.Repo {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf(models.Repo{})))
	return models.Repo{}
}

func AnyPullRequest() models.PullRequest {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf(models.PullRequest{})))
	return models.PullRequest{}
}
