package app

import (
	"context"
	"errors"

	"github.com/pluque01/orza/internal/domain"
)

type FolderService struct {
	repository  FolderRepository
	credentials CredentialLifecycle
}

func NewFolderService(repository FolderRepository, credentials CredentialLifecycle) (*FolderService, error) {
	if repository == nil || credentials == nil {
		return nil, safeFolderUseCaseError("configure folders", "", ErrInvalidRequest)
	}
	return &FolderService{repository: repository, credentials: credentials}, nil
}

func (s *FolderService) Create(ctx context.Context, request CreateFolderRequest) (FolderResult, error) {
	target := selectorTarget(request.Parent)
	if err := contextError(ctx); err != nil {
		return FolderResult{}, safeFolderUseCaseError("create folder", target, err)
	}
	if err := validateSelector(request.Parent); err != nil {
		return FolderResult{}, safeFolderUseCaseError("create folder", target, errors.Join(ErrInvalidRequest, err))
	}
	if request.ExpectedParent != nil && *request.ExpectedParent == 0 {
		return FolderResult{}, safeFolderUseCaseError("create folder", target, ErrInvalidRequest)
	}
	if err := validateExpectedPath(request.ExpectedParentPath); err != nil {
		return FolderResult{}, safeFolderUseCaseError("create folder", target, err)
	}
	if _, err := domain.NewName(request.Name); err != nil {
		return FolderResult{}, safeFolderUseCaseError("create folder", target, errors.Join(ErrInvalidRequest, err))
	}
	result, err := s.repository.CreateFolder(ctx, request)
	if err != nil {
		return FolderResult{}, safeFolderUseCaseError("create folder", target, err)
	}
	return result, nil
}

func (s *FolderService) Get(ctx context.Context, selector ItemSelector) (FolderResult, error) {
	target := selectorTarget(selector)
	if err := contextError(ctx); err != nil {
		return FolderResult{}, safeFolderUseCaseError("get folder", target, err)
	}
	if err := validateSelector(selector); err != nil {
		return FolderResult{}, safeFolderUseCaseError("get folder", target, errors.Join(ErrInvalidRequest, err))
	}
	result, err := s.repository.GetFolder(ctx, selector)
	if err != nil {
		return FolderResult{}, safeFolderUseCaseError("get folder", target, err)
	}
	return result, nil
}

func (s *FolderService) List(ctx context.Context, request ListChildrenRequest) (ListChildrenResult, error) {
	target := selectorTarget(request.Folder)
	if err := contextError(ctx); err != nil {
		return ListChildrenResult{}, safeFolderUseCaseError("list folder", target, err)
	}
	if err := validateSelector(request.Folder); err != nil {
		return ListChildrenResult{}, safeFolderUseCaseError("list folder", target, errors.Join(ErrInvalidRequest, err))
	}
	result, err := s.repository.ListChildren(ctx, request)
	if err != nil {
		return ListChildrenResult{}, safeFolderUseCaseError("list folder", target, err)
	}
	return result, nil
}

func (s *FolderService) Rename(ctx context.Context, request RenameFolderRequest) (FolderResult, error) {
	target := selectorTarget(request.Folder)
	if err := contextError(ctx); err != nil {
		return FolderResult{}, safeFolderUseCaseError("rename folder", target, err)
	}
	if err := validateSelector(request.Folder); err != nil || request.Expected != nil && *request.Expected == 0 {
		return FolderResult{}, safeFolderUseCaseError("rename folder", target, errors.Join(ErrInvalidRequest, err))
	}
	if _, err := domain.NewName(request.Name); err != nil {
		return FolderResult{}, safeFolderUseCaseError("rename folder", target, errors.Join(ErrInvalidRequest, err))
	}
	result, err := s.repository.RenameFolder(ctx, request)
	if err != nil {
		return FolderResult{}, safeFolderUseCaseError("rename folder", target, err)
	}
	return result, nil
}

func (s *FolderService) Move(ctx context.Context, request MoveFolderRequest) (FolderResult, error) {
	target := selectorTarget(request.Folder)
	if err := contextError(ctx); err != nil {
		return FolderResult{}, safeFolderUseCaseError("move folder", target, err)
	}
	if err := validateSelector(request.Folder); err != nil || request.Expected != nil && *request.Expected == 0 || request.ExpectedDestination != nil && *request.ExpectedDestination == 0 {
		return FolderResult{}, safeFolderUseCaseError("move folder", target, errors.Join(ErrInvalidRequest, err))
	}
	if err := validateSelector(request.Destination); err != nil {
		return FolderResult{}, safeFolderUseCaseError("move folder", selectorTarget(request.Destination), errors.Join(ErrInvalidRequest, err))
	}
	if err := validateExpectedPath(request.ExpectedSourcePath); err != nil {
		return FolderResult{}, safeFolderUseCaseError("move folder", target, err)
	}
	if err := validateExpectedPath(request.ExpectedDestinationPath); err != nil {
		return FolderResult{}, safeFolderUseCaseError("move folder", selectorTarget(request.Destination), err)
	}
	result, err := s.repository.MoveFolder(ctx, request)
	if err != nil {
		return FolderResult{}, safeFolderUseCaseError("move folder", target, err)
	}
	return result, nil
}
