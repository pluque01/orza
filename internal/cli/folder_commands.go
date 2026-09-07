package cli

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/domain"
	"github.com/spf13/cobra"
)

type folderProjection struct {
	ID        app.NodeID   `json:"id"`
	Kind      app.NodeKind `json:"kind"`
	Name      string       `json:"name"`
	Path      string       `json:"path"`
	Revision  app.Revision `json:"revision"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

func newFolderCommand() *cobra.Command {
	return &cobra.Command{
		Use: "folder", Short: "Manage catalog folders", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			return NewError(CodeUsage, "a folder command is required", "", nil)
		},
	}
}

func newFolderCreateCommand(service *app.FolderService, options *Options) *cobra.Command {
	return &cobra.Command{
		Use: "create PATH", Short: "Create a folder", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parent, name, err := splitConnectionPath(args[0])
			if err != nil {
				return usageError("invalid folder path", args[0], err)
			}
			if service == nil {
				return NewError(CodeCatalog, "folder service is unavailable", args[0], nil)
			}
			result, err := service.Create(cmd.Context(), app.CreateFolderRequest{Parent: app.ItemSelector{Path: parent}, Name: name})
			if err != nil {
				return folderCommandError(err, args[0])
			}
			return writeFolderResult(cmd, options.JSON, result, "created")
		},
	}
}

func newFolderListCommand(service *app.FolderService, options *Options) *cobra.Command {
	return &cobra.Command{
		Use: "list [PATH_OR_ID]", Short: "List direct folder children", Args: maximumArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "/"
			if len(args) == 1 {
				target = args[0]
			}
			selector, err := parseSelector(target)
			if err != nil {
				return usageError("invalid folder path or ID", target, err)
			}
			if service == nil {
				return NewError(CodeCatalog, "folder service is unavailable", target, nil)
			}
			result, err := service.List(cmd.Context(), app.ListChildrenRequest{Folder: selector})
			if err != nil {
				return folderCommandError(err, target)
			}
			if options.JSON {
				items := make([]any, 0, len(result.Folders)+len(result.Connections))
				for _, folder := range result.Folders {
					items = append(items, projectFolder(folder))
				}
				for _, connection := range result.Connections {
					items = append(items, projectConnection(connection))
				}
				return WriteSuccess(cmd.OutOrStdout(), true, items, &result.CatalogRevision)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(writer, "ID\tKIND\tPATH\tREVISION"); err != nil {
				return err
			}
			for _, folder := range result.Folders {
				if _, err := fmt.Fprintf(writer, "%s\tfolder\t%s\t%d\n", folder.ID, folder.Path, folder.Revision); err != nil {
					return err
				}
			}
			for _, connection := range result.Connections {
				if _, err := fmt.Fprintf(writer, "%s\tconnection\t%s\t%d\n", connection.ID, connection.Path, connection.Revision); err != nil {
					return err
				}
			}
			return writer.Flush()
		},
	}
}

func newFolderShowCommand(service *app.FolderService, options *Options) *cobra.Command {
	return &cobra.Command{
		Use: "show PATH_OR_ID", Short: "Show a folder", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selector, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid folder path or ID", args[0], err)
			}
			if service == nil {
				return NewError(CodeCatalog, "folder service is unavailable", args[0], nil)
			}
			result, err := service.Get(cmd.Context(), selector)
			if err != nil {
				return folderCommandError(err, args[0])
			}
			if options.JSON {
				return WriteSuccess(cmd.OutOrStdout(), true, projectFolder(result.Folder), &result.CatalogRevision)
			}
			return WriteSuccess(cmd.OutOrStdout(), false, fmt.Sprintf("ID: %s\nPath: %s\nRevision: %d", result.Folder.ID, result.Folder.Path, result.Folder.Revision), nil)
		},
	}
}

func newFolderRenameCommand(service *app.FolderService, options *Options) *cobra.Command {
	var expected uint64
	command := &cobra.Command{
		Use: "rename PATH_OR_ID NAME", Short: "Rename a folder", Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			folder, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid folder path or ID", args[0], err)
			}
			if _, err := domain.NewName(args[1]); err != nil {
				return usageError("invalid folder name", args[0], err)
			}
			request := app.RenameFolderRequest{Folder: folder, Name: args[1]}
			if cmd.Flags().Changed("if-revision") {
				if expected == 0 {
					return usageError("revision must be greater than zero", args[0], domain.ErrInvalidRevision)
				}
				revision := app.Revision(expected)
				request.Expected = &revision
			}
			if service == nil {
				return NewError(CodeCatalog, "folder service is unavailable", args[0], nil)
			}
			result, err := service.Rename(cmd.Context(), request)
			if err != nil {
				return folderCommandError(err, args[0])
			}
			return writeFolderResult(cmd, options.JSON, result, "renamed")
		},
	}
	command.Flags().Uint64Var(&expected, "if-revision", 0, "require this folder revision")
	return command
}

func newFolderMoveCommand(service *app.FolderService, options *Options) *cobra.Command {
	var expected uint64
	command := &cobra.Command{
		Use: "move PATH_OR_ID DESTINATION_FOLDER", Short: "Move a folder", Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			folder, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid folder path or ID", args[0], err)
			}
			destination, err := parseSelector(args[1])
			if err != nil {
				return usageError("invalid destination path or ID", args[1], err)
			}
			request := app.MoveFolderRequest{Folder: folder, Destination: destination}
			if cmd.Flags().Changed("if-revision") {
				if expected == 0 {
					return usageError("revision must be greater than zero", args[0], domain.ErrInvalidRevision)
				}
				revision := app.Revision(expected)
				request.Expected = &revision
			}
			if service == nil {
				return NewError(CodeCatalog, "folder service is unavailable", args[0], nil)
			}
			result, err := service.Move(cmd.Context(), request)
			if err != nil {
				return folderCommandError(err, args[0])
			}
			return writeFolderResult(cmd, options.JSON, result, "moved")
		},
	}
	command.Flags().Uint64Var(&expected, "if-revision", 0, "require this folder revision")
	return command
}

func newFolderDeleteCommand(service *app.FolderService, local app.Terminal, options *Options) *cobra.Command {
	var recursive, yes bool
	var expected uint64
	command := &cobra.Command{
		Use: "delete PATH_OR_ID", Short: "Delete a folder", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selector, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid folder path or ID", args[0], err)
			}
			if service == nil {
				return NewError(CodeCatalog, "folder service is unavailable", args[0], nil)
			}
			var requestedRevision *app.Revision
			if cmd.Flags().Changed("if-revision") {
				if expected == 0 {
					return usageError("revision must be greater than zero", args[0], domain.ErrInvalidRevision)
				}
				value := app.Revision(expected)
				requestedRevision = &value
			}

			request := app.DeleteFolderRequest{Folder: selector, Expected: requestedRevision}
			if recursive || !yes {
				scope, scopeErr := service.DeleteScope(cmd.Context(), selector)
				if scopeErr != nil {
					return folderCommandError(scopeErr, args[0])
				}
				if requestedRevision != nil && *requestedRevision != scope.Revision {
					return folderCommandError(app.ErrConflict, args[0])
				}
				if !yes {
					if local == nil || !local.Interactive() {
						return NewError(CodeSecurity, "deletion confirmation requires an interactive terminal or --yes", args[0], app.ErrNonInteractive)
					}
					prompt := fmt.Sprintf("Delete %s (%s, %s, %s)? [y/N] ", scope.Path, countNoun(scope.Folders, "folder"), countNoun(scope.Connections, "connection"), countNoun(scope.RememberedCredentials, "remembered credential"))
					confirmed, confirmErr := confirm(cmd, prompt)
					if confirmErr != nil {
						return confirmErr
					}
					if !confirmed {
						return NewError(CodeCanceled, "deletion canceled", args[0], context.Canceled)
					}
				}
				request = scope.Request(recursive)
			}
			result, err := service.Delete(cmd.Context(), request)
			if err != nil {
				return folderCommandError(err, args[0])
			}
			data := struct {
				FoldersDeleted     int `json:"foldersDeleted"`
				ConnectionsDeleted int `json:"connectionsDeleted"`
			}{result.FoldersDeleted, result.ConnectionsDeleted}
			if options.JSON {
				return WriteSuccess(cmd.OutOrStdout(), true, data, &result.CatalogRevision)
			}
			return WriteSuccess(cmd.OutOrStdout(), false, fmt.Sprintf("deleted %d folders and %d connections", result.FoldersDeleted, result.ConnectionsDeleted), nil)
		},
	}
	command.Flags().BoolVar(&recursive, "recursive", false, "delete all folder descendants")
	command.Flags().Uint64Var(&expected, "if-revision", 0, "require this folder revision")
	command.Flags().BoolVar(&yes, "yes", false, "delete without an interactive confirmation")
	return command
}

func projectFolder(folder app.Folder) folderProjection {
	return folderProjection{ID: folder.ID, Kind: folder.Kind, Name: folder.Name, Path: folder.Path, Revision: folder.Revision, CreatedAt: folder.CreatedAt, UpdatedAt: folder.UpdatedAt}
}

func writeFolderResult(cmd *cobra.Command, jsonOutput bool, result app.FolderResult, verb string) error {
	if jsonOutput {
		return WriteSuccess(cmd.OutOrStdout(), true, projectFolder(result.Folder), &result.CatalogRevision)
	}
	return WriteSuccess(cmd.OutOrStdout(), false, fmt.Sprintf("%s %s (%s), revision %d", verb, result.Folder.Path, result.Folder.ID, result.Folder.Revision), nil)
}

func folderCommandError(err error, target string) error {
	code, message := CodeCatalog, "catalog operation failed"
	switch app.ErrorKindOf(err) {
	case app.ErrorKindInvalid:
		code, message = CodeUsage, "request is invalid"
	case app.ErrorKindNotFound:
		code, message = CodeNotFound, "folder was not found"
	case app.ErrorKindConflict:
		code, message = CodeConflict, "the folder or confirmed scope changed; reload before retrying"
	case app.ErrorKindCanceled:
		code, message = CodeCanceled, "operation canceled"
	case app.ErrorKindSecurity:
		code, message = CodeSecurity, "secure operation failed"
	}
	return NewError(code, message, target, err)
}

func countNoun(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
