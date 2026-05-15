package githubapi

import (
	"context"
	"errors"
	"testing"
)

type lazyFakeService struct {
	listCalls int
}

func (f *lazyFakeService) ListStarLists(context.Context) ([]StarList, error) {
	f.listCalls++
	return []StarList{{Name: "Tools", ID: "UL_1", URL: "https://github.com/stars/testuser/lists/tools"}}, nil
}

func (f *lazyFakeService) ListRepositories(context.Context, string) ([]Repository, error) {
	return nil, nil
}

func TestLazyServiceDefersConstructionUntilRuntimeCall(t *testing.T) {
	var constructorCalls int
	fake := &lazyFakeService{}
	service := newLazyService(func() (Service, error) {
		constructorCalls++
		return fake, nil
	})

	if constructorCalls != 0 {
		t.Fatalf("constructor calls after NewLazyService = %d, want 0", constructorCalls)
	}

	lists, err := service.ListStarLists(context.Background())
	if err != nil {
		t.Fatalf("ListStarLists returned error: %v", err)
	}
	if len(lists) != 1 || lists[0].ID != "UL_1" {
		t.Fatalf("ListStarLists() = %#v, want one fake list", lists)
	}
	if constructorCalls != 1 {
		t.Fatalf("constructor calls after first runtime call = %d, want 1", constructorCalls)
	}

	_, err = service.ListStarLists(context.Background())
	if err != nil {
		t.Fatalf("second ListStarLists returned error: %v", err)
	}
	if constructorCalls != 1 {
		t.Fatalf("constructor calls after second runtime call = %d, want 1", constructorCalls)
	}
	if fake.listCalls != 2 {
		t.Fatalf("underlying list calls = %d, want 2", fake.listCalls)
	}
}

func TestLazyServiceReturnsConstructorError(t *testing.T) {
	boom := errors.New("gh auth unavailable")
	service := newLazyService(func() (Service, error) {
		return nil, boom
	})

	lists, err := service.ListStarLists(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("ListStarLists error = %v, want constructor error %v", err, boom)
	}
	if lists != nil {
		t.Fatalf("ListStarLists results = %#v, want nil", lists)
	}
}

func TestLazyServiceDoesNotConstructWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var constructorCalls int
	service := newLazyService(func() (Service, error) {
		constructorCalls++
		return &lazyFakeService{}, nil
	})

	lists, err := service.ListStarLists(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListStarLists error = %v, want context.Canceled", err)
	}
	if lists != nil {
		t.Fatalf("ListStarLists results = %#v, want nil", lists)
	}
	if constructorCalls != 0 {
		t.Fatalf("constructor calls = %d, want 0", constructorCalls)
	}
}
