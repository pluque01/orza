package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/domain"
	"github.com/spf13/cobra"
)

type connectionProjection struct {
	ID           app.NodeID     `json:"id"`
	Kind         app.NodeKind   `json:"kind"`
	Name         string         `json:"name"`
	Path         string         `json:"path"`
	Revision     app.Revision   `json:"revision"`
	Host         string         `json:"host"`
	Port         uint16         `json:"port"`
	Username     string         `json:"user,omitempty"`
	AuthMethod   app.AuthMethod `json:"auth"`
	IdentityFile string         `json:"identityFile,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

func newConnectionListCommand(service *app.ConnectionService, options *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list [FOLDER_PATH_OR_ID]",
		Short: "List direct connection children",
		Args:  maximumArgs(1),
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
				return NewError(CodeCatalog, "connection service is unavailable", target, nil)
			}
			result, err := service.List(cmd.Context(), app.ListConnectionsRequest{Folder: selector})
			if err != nil {
				return commandError(err, target)
			}
			projections := make([]connectionProjection, len(result.Connections))
			for index := range result.Connections {
				projections[index] = projectConnection(result.Connections[index])
			}
			if options.JSON {
				return WriteSuccess(cmd.OutOrStdout(), true, projections, &result.CatalogRevision)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(writer, "ID\tPATH\tTARGET\tAUTH\tREVISION"); err != nil {
				return err
			}
			for _, connection := range result.Connections {
				target := connection.Host + ":" + fmt.Sprint(connection.Port)
				if connection.Username != "" {
					target = connection.Username + "@" + target
				}
				if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%d\n", connection.ID, connection.Path, target, connection.AuthMethod, connection.Revision); err != nil {
					return err
				}
			}
			return writer.Flush()
		},
	}
}

func newConnectionShowCommand(service *app.ConnectionService, options *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "show PATH_OR_ID",
		Short: "Show a connection",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selector, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid connection path or ID", args[0], err)
			}
			if service == nil {
				return NewError(CodeCatalog, "connection service is unavailable", args[0], nil)
			}
			result, err := service.Get(cmd.Context(), selector)
			if err != nil {
				return commandError(err, args[0])
			}
			projection := projectConnection(result.Connection)
			if options.JSON {
				return WriteSuccess(cmd.OutOrStdout(), true, projection, &result.CatalogRevision)
			}
			identity := ""
			if result.Connection.IdentityFile != "" {
				identity = "\nIdentity file: " + result.Connection.IdentityFile

			}
			user := result.Connection.Username
			if user == "" {
				user = "(default)"
			}
			text := fmt.Sprintf("ID: %s\nPath: %s\nRevision: %d\nHost: %s\nPort: %d\nUser: %s\nAuth: %s%s",
				result.Connection.ID, result.Connection.Path, result.Connection.Revision, result.Connection.Host,
				result.Connection.Port, user, result.Connection.AuthMethod, identity)
			return WriteSuccess(cmd.OutOrStdout(), false, text, nil)
		},
	}
}

func projectConnection(connection app.Connection) connectionProjection {
	return connectionProjection{
		ID: connection.ID, Kind: connection.Kind, Name: connection.Name, Path: connection.Path,
		Revision: connection.Revision, Host: connection.Host, Port: connection.Port,
		Username: connection.Username, AuthMethod: connection.AuthMethod, IdentityFile: connection.IdentityFile,
		CreatedAt: connection.CreatedAt, UpdatedAt: connection.UpdatedAt,
	}
}

func parseSelector(value string) (app.ItemSelector, error) {
	if strings.HasPrefix(value, "/") {
		path, err := domain.ParseLogicalPath(value)
		if err != nil {
			return app.ItemSelector{}, err
		}
		return app.ItemSelector{Path: path.String()}, nil
	}
	id, err := domain.ParseID(value)
	if err != nil {
		return app.ItemSelector{}, err
	}
	return app.ItemSelector{ID: app.NodeID(id.String())}, nil
}
