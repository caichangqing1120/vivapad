package syntax

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	udlBundleVersion  = 1
	MaxUDLBundleBytes = 2 << 20
)

func BuiltinUDLPresets() []UDL {
	return []UDL{
		{
			Name: "Nginx", Extensions: []string{"conf", "nginx"}, LineComment: "#", Quotes: []string{`"`, "'"},
			Keywords: strings.Fields("worker_processes worker_connections events http server listen server_name location root index include upstream proxy_pass proxy_set_header proxy_redirect return rewrite try_files error_page access_log error_log log_format add_header set map if limit_req gzip ssl_certificate ssl_certificate_key"),
		},
		{
			Name: "Dockerfile", Extensions: []string{"dockerfile"}, LineComment: "#", Quotes: []string{`"`, "'"},
			Keywords: strings.Fields("FROM RUN CMD COPY ADD ENV ARG EXPOSE WORKDIR ENTRYPOINT VOLUME USER LABEL HEALTHCHECK AS ONBUILD SHELL STOPSIGNAL"),
		},
		{
			Name: "Log / StackTrace", Extensions: []string{"log"},
			Keywords: strings.Fields("ERROR WARN WARNING INFO DEBUG TRACE FATAL PANIC EXCEPTION CAUSED AT TIMEOUT FAILED Exception Caused at"),
		},
		{
			Name: "GraphQL", Extensions: []string{"graphql", "gql"}, LineComment: "#", Quotes: []string{`"""`, `"`},
			Keywords: strings.Fields("query mutation subscription fragment on type interface union enum input scalar extend implements directive schema repeatable true false null"),
		},
		{
			Name: "Prisma", Extensions: []string{"prisma"}, LineComment: "//", Quotes: []string{`"`},
			Keywords: strings.Fields("model enum datasource generator type view String Int BigInt Float Boolean DateTime Json Bytes Decimal Unsupported"),
		},
		{
			Name: "Protobuf", Extensions: []string{"proto"}, LineComment: "//", BlockStart: "/*", BlockEnd: "*/", Quotes: []string{`"`, "'"},
			Keywords: strings.Fields("syntax edition package import option message enum service rpc returns repeated optional required oneof map reserved extensions extend public weak stream"),
		},
		{
			Name: "Makefile", Extensions: []string{"mk", "make"}, LineComment: "#",
			Keywords: strings.Fields("include ifeq ifneq ifdef ifndef else endif define endef export unexport override .PHONY"),
		},
		{
			Name: "Env / Properties", Extensions: []string{"env", "properties", "cfg"}, LineComment: "#", Quotes: []string{`"`, "'"},
			Keywords: strings.Fields("true false null yes no on off"),
		},
	}
}

type udlBundle struct {
	Version int   `json:"version"`
	UDLs    []UDL `json:"udls"`
}

func ExportUDLBundle(definitions []UDL) ([]byte, error) {
	normalized, err := normalizeUDLBundle(definitions)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(udlBundle{Version: udlBundleVersion, UDLs: normalized}, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(data)+1 > MaxUDLBundleBytes {
		return nil, fmt.Errorf("UDL bundle exceeds 2 MiB")
	}
	return append(data, '\n'), nil
}

func ParseUDLBundle(data []byte) ([]UDL, error) {
	if len(data) > MaxUDLBundleBytes {
		return nil, fmt.Errorf("UDL bundle exceeds 2 MiB")
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("UDL bundle must be UTF-8 JSON")
	}
	var bundle udlBundle
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return nil, fmt.Errorf("invalid UDL bundle JSON: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("UDL bundle has trailing JSON data")
	}
	if err := validateUDLJSON(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return nil, fmt.Errorf("invalid UDL bundle JSON: %w", err)
	}
	if bundle.Version != udlBundleVersion {
		return nil, fmt.Errorf("unsupported UDL bundle version %d; expected version %d", bundle.Version, udlBundleVersion)
	}
	return normalizeUDLBundle(bundle.UDLs)
}

func validateUDLJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case nil:
		return fmt.Errorf("null is not allowed")
	case json.Delim('{'), json.Delim('['):
		var fields map[string]bool
		if token == json.Delim('{') {
			fields = make(map[string]bool)
		}
		for decoder.More() {
			if fields != nil {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				folded := strings.ToLower(key.(string))
				if fields[folded] {
					return fmt.Errorf("duplicate field %q", key)
				}
				fields[folded] = true
			}
			if err := validateUDLJSON(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
	}
	return err
}

func normalizeUDLBundle(definitions []UDL) ([]UDL, error) {
	if len(definitions) == 0 {
		return nil, fmt.Errorf("UDL bundle must contain at least one definition")
	}
	normalized, err := normalizeUDLs(definitions)
	if err != nil {
		return nil, err
	}
	for _, definition := range normalized {
		for _, value := range []string{definition.Name, definition.LineComment, definition.BlockStart, definition.BlockEnd} {
			if !utf8.ValidString(value) {
				return nil, fmt.Errorf("UDL name and delimiters must be UTF-8")
			}
		}
		for _, quote := range definition.Quotes {
			if !utf8.ValidString(quote) {
				return nil, fmt.Errorf("UDL string delimiters must be UTF-8")
			}
		}
	}
	return normalized, nil
}

type ConflictPolicy uint8

const (
	ConflictSkip ConflictPolicy = iota
	ConflictOverwrite
	ConflictRename
)

type MergeReport struct {
	Imported, Skipped, Overwritten, Renamed int
	UnassignedExtensions                    int
}

func MergeUDLs(existing, incoming []UDL, policy ConflictPolicy) ([]UDL, MergeReport, error) {
	var report MergeReport
	if policy > ConflictRename {
		return nil, report, fmt.Errorf("unknown UDL conflict policy %d", policy)
	}
	merged, err := normalizeUDLs(existing)
	if err != nil {
		return nil, report, fmt.Errorf("existing UDLs: %w", err)
	}
	additions, err := normalizeUDLs(incoming)
	if err != nil {
		return nil, report, fmt.Errorf("incoming UDLs: %w", err)
	}
	indices, reserved := make(map[string]int), make(map[string]bool)
	for index, definition := range merged {
		indices[definition.Mode()] = index
		reserved[definition.Mode()] = true
	}
	for _, definition := range additions {
		reserved[definition.Mode()] = true
	}
	for _, definition := range additions {
		if index, conflict := indices[definition.Mode()]; conflict {
			switch policy {
			case ConflictSkip:
				report.Skipped++
				continue
			case ConflictOverwrite:
				merged[index] = definition
				report.Overwritten++
				continue
			case ConflictRename:
				definition.Name = renamedUDLName(definition.Name, reserved)
				reserved[definition.Mode()] = true
				definition.Extensions = slices.DeleteFunc(definition.Extensions, func(extension string) bool {
					for _, registered := range merged {
						if slices.Contains(registered.Extensions, extension) {
							report.UnassignedExtensions++
							return true
						}
					}
					return false
				})
				report.Renamed++
			}
		} else {
			report.Imported++
		}
		merged = append(merged, definition)
	}
	merged, err = normalizeUDLs(merged)
	if err != nil {
		return nil, MergeReport{}, fmt.Errorf("merged UDLs: %w", err)
	}
	for index := range merged {
		merged[index].Keywords = slices.Clone(merged[index].Keywords)
		merged[index].Quotes = slices.Clone(merged[index].Quotes)
	}
	return merged, report, nil
}

func renamedUDLName(name string, reserved map[string]bool) string {
	for number := 2; ; number++ {
		suffix := fmt.Sprintf(" (%d)", number)
		base := name
		for len(base)+len(suffix) > maxUDLNameBytes {
			_, size := utf8.DecodeLastRuneInString(base)
			base = base[:len(base)-size]
		}
		candidate := UDL{Name: strings.TrimSpace(base) + suffix}
		if !reserved[candidate.Mode()] {
			return candidate.Name
		}
	}
}
