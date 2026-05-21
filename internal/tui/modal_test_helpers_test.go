package tui

import (
	"context"
	"sync"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

type recordingFakeService struct {
	fakeService
	createCalls []githubapi.StarListInput
	updateCalls []githubapi.UpdateStarListInput
	deleteCalls []string
	createErr   error
	updateErr   error
	deleteErr   error
}

func (f *recordingFakeService) CreateStarList(
	_ context.Context, input githubapi.StarListInput,
) (githubapi.StarList, error) {
	f.createCalls = append(f.createCalls, input)
	return githubapi.StarList{Name: input.Name}, f.createErr
}

func (f *recordingFakeService) UpdateStarList(
	_ context.Context, input githubapi.UpdateStarListInput,
) (githubapi.StarList, error) {
	f.updateCalls = append(f.updateCalls, input)
	return githubapi.StarList{}, f.updateErr
}

func (f *recordingFakeService) DeleteStarList(_ context.Context, id string) error {
	f.deleteCalls = append(f.deleteCalls, id)
	return f.deleteErr
}

type repoMutationFakeService struct {
	fakeService
	mu                sync.Mutex
	membershipsResult struct {
		repoID  string
		listIDs []string
		err     error
	}
	membershipsCalls []string
	updateListsCalls []struct {
		repoID  string
		listIDs []string
	}
	removeStarCalls []string
	removeStarErr   error
}

func (f *repoMutationFakeService) GetRepositoryMemberships(
	_ context.Context, nameWithOwner string,
) (string, []string, error) {
	f.mu.Lock()
	f.membershipsCalls = append(f.membershipsCalls, nameWithOwner)
	f.mu.Unlock()
	return f.membershipsResult.repoID, f.membershipsResult.listIDs, f.membershipsResult.err
}

func (f *repoMutationFakeService) UpdateRepositoryLists(
	_ context.Context, repoID string, listIDs []string,
) error {
	f.mu.Lock()
	f.updateListsCalls = append(f.updateListsCalls, struct {
		repoID  string
		listIDs []string
	}{repoID, listIDs})
	f.mu.Unlock()
	return nil
}

func (f *repoMutationFakeService) RemoveStar(_ context.Context, repoID string) error {
	f.removeStarCalls = append(f.removeStarCalls, repoID)
	return f.removeStarErr
}

type copyMergeFakeService struct {
	fakeService
	reposResult        []githubapi.Repository
	membershipsRepoID  string
	membershipsListIDs []string
	updateListsCalls   [][]string // just listIDs per call
	deleteListCalls    []string
	deleteListErr      error
}

func (f *copyMergeFakeService) ListRepositories(
	_ context.Context, _ string, _ ...githubapi.ListOptions,
) ([]githubapi.Repository, error) {
	return f.reposResult, nil
}

func (f *copyMergeFakeService) GetRepositoryMemberships(
	_ context.Context, _ string,
) (string, []string, error) {
	return f.membershipsRepoID, f.membershipsListIDs, nil
}

func (f *copyMergeFakeService) UpdateRepositoryLists(
	_ context.Context, _ string, listIDs []string,
) error {
	f.updateListsCalls = append(f.updateListsCalls, listIDs)
	return nil
}

func (f *copyMergeFakeService) DeleteStarList(_ context.Context, id string) error {
	f.deleteListCalls = append(f.deleteListCalls, id)
	return f.deleteListErr
}
