// Package vulnpolicy applies Orza's temporary-exception policy to govulncheck
// streaming JSON without weakening scanner failures.
package vulnpolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	maxExceptionBytes   = 1 << 20
	maxScannerMessage   = 4 << 20
	maxScannerMessages  = 100000
	maxVulnerabilityIDs = 10000
	maxDiagnosticLines  = 100
)

var vulnerabilityIDPattern = regexp.MustCompile(`^GO-[0-9]{4}-[0-9]{4}$`)

// Exception is one reviewed, temporary exception for an exact reachable ID.
type Exception struct {
	ID         string `json:"id"`
	Owner      string `json:"owner"`
	Rationale  string `json:"rationale"`
	Mitigation string `json:"mitigation"`
	Issue      string `json:"issue"`
	Expires    string `json:"expires"`
}

type scannerFinding struct {
	OSV   string         `json:"osv"`
	Trace []scannerFrame `json:"trace"`
}

type scannerFrame struct {
	Package  string `json:"package"`
	Function string `json:"function"`
}

type scannerOSV struct {
	ID string `json:"id"`
}

type diagnostics struct {
	w     io.Writer
	lines int
	err   error
}

func (d *diagnostics) printf(format string, args ...any) {
	if d.err != nil || d.lines >= maxDiagnosticLines {
		return
	}
	_, d.err = fmt.Fprintf(d.w, format+"\n", args...)
	d.lines++
}

// Evaluate validates exceptions and scanner protocol, writes only bounded safe
// diagnostics, and returns an error whenever the scan must block.
func Evaluate(scannerStream, exceptionJSON io.Reader, now time.Time, output io.Writer) error {
	exceptions, validationProblems := decodeExceptions(exceptionJSON, now)
	d := diagnostics{w: output}
	for _, problem := range validationProblems {
		d.printf("failure: vulnerability exception %s", problem)
	}
	if len(validationProblems) != 0 {
		return policyError(len(validationProblems), d.err)
	}

	reachable, observed, protocolProblems := decodeScanner(scannerStream)
	for _, problem := range protocolProblems {
		d.printf("failure: scanner protocol %s", problem)
	}
	if len(protocolProblems) != 0 {
		return policyError(len(protocolProblems), d.err)
	}

	problems := 0
	exceptionIDs := make(map[string]struct{}, len(exceptions))
	for id := range exceptions {
		exceptionIDs[id] = struct{}{}
	}
	for _, id := range sortedIDs(exceptionIDs) {
		exception := exceptions[id]
		if _, ok := reachable[id]; !ok {
			d.printf("failure: stale vulnerability exception for %s", id)
			problems++
			continue
		}
		d.printf("warning: active vulnerability exception %s owner=%s issue=%s expires=%s", id, exception.Owner, exception.Issue, exception.Expires)
	}

	for _, id := range sortedIDs(reachable) {
		if _, ok := exceptions[id]; !ok {
			d.printf("failure: reachable vulnerability %s has no active exception", id)
			problems++
		}
	}
	for _, id := range sortedIDs(observed) {
		if _, ok := reachable[id]; !ok {
			d.printf("info: unreachable vulnerability %s", id)
		}
	}
	if problems != 0 || d.err != nil {
		return policyError(problems, d.err)
	}
	return nil
}

func decodeExceptions(input io.Reader, now time.Time) (map[string]Exception, []string) {
	limited := io.LimitReader(input, maxExceptionBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, []string{"file could not be read"}
	}
	if len(data) > maxExceptionBytes {
		return nil, []string{"file exceeds size limit"}
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var records []Exception
	if err := decoder.Decode(&records); err != nil {
		return nil, []string{"file is malformed"}
	}
	if records == nil {
		return nil, []string{"file must contain a JSON array"}
	}
	if len(records) > maxVulnerabilityIDs {
		return nil, []string{"file contains too many records"}
	}
	if err := requireEOF(decoder); err != nil {
		return nil, []string{"file contains trailing data"}
	}

	result := make(map[string]Exception, len(records))
	var problems []string
	today := now.UTC().Format(time.DateOnly)
	for index, record := range records {
		label := fmt.Sprintf("record %d", index+1)
		if !vulnerabilityIDPattern.MatchString(record.ID) {
			problems = append(problems, label+" has an invalid id")
		}
		if !validText(record.Owner, 128) {
			problems = append(problems, label+" has an invalid owner")
		}
		if !validText(record.Rationale, 2048) {
			problems = append(problems, label+" has an invalid rationale")
		}
		if !validText(record.Mitigation, 2048) {
			problems = append(problems, label+" has an invalid mitigation")
		}
		if !validHTTPSURL(record.Issue) {
			problems = append(problems, label+" has an invalid issue URL")
		}
		expires, err := time.Parse(time.DateOnly, record.Expires)
		if err != nil || expires.Format(time.DateOnly) != record.Expires {
			problems = append(problems, label+" has an invalid expiration date")
		} else if record.Expires <= today {
			problems = append(problems, label+" is expired")
		}
		if _, duplicate := result[record.ID]; duplicate {
			problems = append(problems, label+" duplicates an id")
		}
		result[record.ID] = record
	}
	return result, problems
}

func decodeScanner(input io.Reader) (map[string]struct{}, map[string]struct{}, []string) {
	reachable := make(map[string]struct{})
	observed := make(map[string]struct{})
	decoder := json.NewDecoder(input)
	messageCount := 0
	configCount := 0
	for {
		var encoded json.RawMessage
		if err := decoder.Decode(&encoded); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, nil, []string{"contains malformed JSON"}
		}
		messageCount++
		if messageCount > maxScannerMessages {
			return nil, nil, []string{"message count exceeds limit"}
		}
		if len(encoded) > maxScannerMessage {
			return nil, nil, []string{"message exceeds size limit"}
		}
		var message map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &message); err != nil {
			return nil, nil, []string{"contains malformed JSON"}
		}
		if len(message) != 1 {
			return nil, nil, []string{"message must contain exactly one event"}
		}
		for kind, raw := range message {
			switch kind {
			case "config", "progress", "SBOM":
				if string(raw) == "null" {
					return nil, nil, []string{kind + " event is null"}
				}
				if kind == "config" {
					configCount++
					if configCount > 1 {
						return nil, nil, []string{"contains duplicate config events"}
					}
				}
			case "osv":
				var entry scannerOSV
				if err := json.Unmarshal(raw, &entry); err != nil || !vulnerabilityIDPattern.MatchString(entry.ID) {
					return nil, nil, []string{"osv event is malformed"}
				}
				observed[entry.ID] = struct{}{}
			case "finding":
				var finding scannerFinding
				if err := json.Unmarshal(raw, &finding); err != nil || !vulnerabilityIDPattern.MatchString(finding.OSV) || len(finding.Trace) == 0 {
					return nil, nil, []string{"finding event is malformed"}
				}
				observed[finding.OSV] = struct{}{}
				for _, frame := range finding.Trace {
					if frame.Package != "" || frame.Function != "" {
						reachable[finding.OSV] = struct{}{}
						break
					}
				}
			default:
				return nil, nil, []string{"contains unknown event type"}
			}
		}
		if len(observed) > maxVulnerabilityIDs {
			return nil, nil, []string{"vulnerability count exceeds limit"}
		}
	}
	if configCount != 1 {
		return nil, nil, []string{"does not contain a config event"}
	}
	return reachable, observed, nil
}

func requireEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return errors.New("trailing data")
}

func validText(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum {
		return false
	}
	return !strings.ContainsAny(value, "\r\n\x00")
}

func validHTTPSURL(value string) bool {
	if !validText(value, 2048) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func sortedIDs(ids map[string]struct{}) []string {
	result := make([]string, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func policyError(problems int, outputErr error) error {
	if outputErr != nil {
		return errors.New("vulnerability policy could not write diagnostics")
	}
	return fmt.Errorf("vulnerability policy rejected scan with %d problem(s)", problems)
}
