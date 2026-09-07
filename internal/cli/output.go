package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/pluque01/orza/internal/app"
)

type successEnvelope struct {
	OK              bool                 `json:"ok"`
	Data            any                  `json:"data"`
	CatalogRevision *app.CatalogRevision `json:"catalogRevision,omitempty"`
}

type errorEnvelope struct {
	OK    bool          `json:"ok"`
	Error errorResponse `json:"error"`
}

type errorResponse struct {
	Code            ErrorCode            `json:"code"`
	Message         string               `json:"message"`
	Target          string               `json:"target,omitempty"`
	Endpoint        string               `json:"endpoint,omitempty"`
	Category        app.SSHFailureReason `json:"category,omitempty"`
	Stage           app.SSHFailureStage  `json:"stage,omitempty"`
	Recommendation  string               `json:"recommendation,omitempty"`
	TechnicalDetail string               `json:"technicalDetail,omitempty"`
}

// WriteSuccess emits exactly one JSON object in JSON mode, or one human-readable
// line for values with a textual representation.
func WriteSuccess(w io.Writer, jsonOutput bool, data any, revision *app.CatalogRevision) error {
	if jsonOutput {
		return json.NewEncoder(w).Encode(successEnvelope{
			OK:              true,
			Data:            data,
			CatalogRevision: revision,
		})
	}
	if data == nil {
		return nil
	}
	_, err := fmt.Fprintln(w, data)
	return err
}

// WriteError renders only classified, safe fields. Internal causes are never
// formatted and therefore cannot leak secret material through process output.
func WriteError(w io.Writer, jsonOutput bool, err error) error {
	code, message, target := safeError(err)
	endpoint, diagnostic := startupErrorFields(err)
	if jsonOutput {
		response := errorResponse{Code: code, Message: message, Target: target, Endpoint: endpoint}
		if diagnostic != nil {
			response.Category = diagnostic.Category
			response.Stage = diagnostic.Stage
			response.Recommendation = diagnostic.Recommendation
			response.TechnicalDetail = diagnostic.TechnicalDetail
		}
		return json.NewEncoder(w).Encode(errorEnvelope{
			OK:    false,
			Error: response,
		})
	}

	if diagnostic != nil {
		if _, writeErr := fmt.Fprintln(w, "error: SSH session could not start"); writeErr != nil {
			return writeErr
		}
		if _, writeErr := fmt.Fprintf(w, "summary: %s\n", diagnostic.Summary); writeErr != nil {
			return writeErr
		}
		if target != "" {
			if _, writeErr := fmt.Fprintf(w, "target: %s\n", target); writeErr != nil {
				return writeErr
			}
		}
		if endpoint != "" {
			if _, writeErr := fmt.Fprintf(w, "endpoint: %s\n", endpoint); writeErr != nil {
				return writeErr
			}
		}
		if _, writeErr := fmt.Fprintf(w, "category: %s\nstage: %s\nrecommendation: %s\n", diagnostic.Category, diagnostic.Stage, diagnostic.Recommendation); writeErr != nil {
			return writeErr
		}
		if diagnostic.TechnicalDetail != "" {
			_, writeErr := fmt.Fprintf(w, "detail: %s\n", diagnostic.TechnicalDetail)
			return writeErr
		}
		return nil
	}

	if target == "" {
		_, writeErr := fmt.Fprintf(w, "error: %s\n", message)
		return writeErr
	}
	_, writeErr := fmt.Fprintf(w, "error: %s (target: %s)\n", message, target)
	return writeErr
}
