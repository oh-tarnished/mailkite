// protodoc generates documentation for a tree of protobuf definitions.
//
// For every module directory (a directory whose version sub-dirs — v1/, v2/ …
// — contain .proto files) it writes a per-module README.md describing the
// services, RPCs, messages, and enums found there. It then writes a single
// top-level README.md summarising every module and rendering a Mermaid graph
// of the local import relationships between them (see summary.go).
//
// Usage:
//
//	go run ./tools/protobuf/docs [protobuf-dir]
//
// If no argument is given, protobuf-dir defaults to "protobuf".
//
// All generated files carry an "auto-generated, do not edit" banner and are
// overwritten on every run. The Markdown is emitted in a markdownlint-clean
// style (padded table pipes, no blank lines inside blockquotes).
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// --- Data Model ---

type Module struct {
	Name     string   // e.g., "Interaction"
	Package  string   // e.g., "engine.interaction.v1"
	Dir      string   // path to module dir (relative to the CWD, as walked)
	Imports  []string // raw import paths collected from all .proto files
	Services []Service
	Messages []Message
	Enums    []Enum
}

type Service struct {
	Name    string
	Comment string
	RPCs    []RPC
}

type RPC struct {
	Name         string
	Comment      string
	Request      string
	Response     string
	ServerStream bool
}

type Message struct {
	Name    string
	Comment string
	Fields  []Field
}

type Field struct {
	Name     string
	Type     string
	Comment  string
	Required bool   // true if REQUIRED or IDENTIFIER
	Repeated bool   // true for `repeated` fields (rendered as a list)
	Behavior string // e.g. REQUIRED, OPTIONAL, OUTPUT_ONLY, IDENTIFIER
}

type Enum struct {
	Name    string
	Comment string
	Values  []EnumValue
}

type EnumValue struct {
	Name    string
	Number  string
	Comment string
}

// --- Regex patterns ---

var (
	rePackage       = regexp.MustCompile(`^package\s+([\w.]+)\s*;`)
	reImport        = regexp.MustCompile(`^import\s+"([^"]+)"\s*;`)
	reService       = regexp.MustCompile(`^service\s+(\w+)\s*\{`)
	reRPC           = regexp.MustCompile(`^\s*rpc\s+(\w+)\s*\(\s*(\w+)\s*\)\s*returns\s*\(\s*(stream\s+)?(\w+)\s*\)`)
	reMessage       = regexp.MustCompile(`^message\s+(\w+)\s*\{`)
	reEnum          = regexp.MustCompile(`^enum\s+(\w+)\s*\{`)
	reField         = regexp.MustCompile(`^\s*(?:optional\s+|repeated\s+)?(\S+)\s+(\w+)\s*=\s*\d+`)
	reEnumVal       = regexp.MustCompile(`^\s*(\w+)\s*=\s*(\d+)`)
	reComment       = regexp.MustCompile(`^\s*//\s?(.*)`)
	reFieldBehavior = regexp.MustCompile(`\(google\.api\.field_behavior\)\s*=\s*(\w+)`)
)

func main() {
	protoDir := "protobuf"
	if len(os.Args) > 1 {
		protoDir = os.Args[1]
	}

	modules, err := discoverModules(protoDir)
	if err != nil {
		log.Fatalf("Failed to discover modules: %v", err)
	}

	for _, mod := range modules {
		if err := generateReadme(mod); err != nil {
			log.Printf("Warning: failed to generate README for %s: %v", mod.Dir, err)
			continue
		}
		log.Printf("Generated README.md for %s (%d services, %d messages, %d enums)",
			mod.Package, len(mod.Services), len(mod.Messages), len(mod.Enums))
	}

	// Top-level summary + dependency graph spanning every module.
	if err := generateRootReadme(protoDir, modules); err != nil {
		log.Printf("Warning: failed to generate root README: %v", err)
	} else {
		log.Printf("Generated root summary README.md for %s/", protoDir)
	}

	log.Printf("Done. Generated %d module READMEs + 1 root summary.", len(modules))
}

// discoverModules finds all module directories under protoDir.
// A module directory is one that contains version subdirs (v1/, v2/) with .proto files.
func discoverModules(protoDir string) ([]Module, error) {
	var modules []Module

	err := filepath.Walk(protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}

		// Look for version sub-directories containing .proto files.
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}

		var protoFiles []string
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), "v") {
				continue
			}
			versionDir := filepath.Join(path, e.Name())
			files, _ := filepath.Glob(filepath.Join(versionDir, "*.proto"))
			protoFiles = append(protoFiles, files...)
		}

		if len(protoFiles) == 0 {
			return nil
		}

		mod := Module{Dir: path}
		sort.Strings(protoFiles)

		for _, pf := range protoFiles {
			if err := parseProtoFile(pf, &mod); err != nil {
				log.Printf("Warning: failed to parse %s: %v", pf, err)
			}
		}

		// Derive module name from directory.
		rel, _ := filepath.Rel(protoDir, path)
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) > 0 {
			last := parts[len(parts)-1]
			mod.Name = titleCase(last)
		}

		modules = append(modules, mod)
		return filepath.SkipDir
	})

	return modules, err
}

// parseProtoFile parses a single .proto file and appends to the module.
func parseProtoFile(path string, mod *Module) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // read-only; close error is not meaningful

	scanner := bufio.NewScanner(f)
	var commentBuf []string
	braceDepth := 0
	var currentService *Service
	var currentMessage *Message
	var currentEnum *Enum

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track brace depth.
		braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

		// Collect comments.
		if m := reComment.FindStringSubmatch(trimmed); m != nil {
			commentBuf = append(commentBuf, m[1])
			continue
		}

		comment := strings.TrimSpace(strings.Join(commentBuf, " "))

		// Import (used to build the cross-module dependency graph).
		if m := reImport.FindStringSubmatch(trimmed); m != nil {
			mod.Imports = append(mod.Imports, m[1])
			commentBuf = nil
			continue
		}

		// Package.
		if m := rePackage.FindStringSubmatch(trimmed); m != nil {
			if mod.Package == "" {
				mod.Package = m[1]
			}
			commentBuf = nil
			continue
		}

		// Service declaration.
		if m := reService.FindStringSubmatch(trimmed); m != nil {
			svc := Service{Name: m[1], Comment: comment}
			mod.Services = append(mod.Services, svc)
			currentService = &mod.Services[len(mod.Services)-1]
			currentMessage = nil
			currentEnum = nil
			commentBuf = nil
			continue
		}

		// RPC declaration (inside service).
		if currentService != nil && braceDepth >= 1 {
			if m := reRPC.FindStringSubmatch(trimmed); m != nil {
				rpc := RPC{
					Name:         m[1],
					Request:      m[2],
					Response:     m[4],
					Comment:      comment,
					ServerStream: m[3] != "",
				}
				currentService.RPCs = append(currentService.RPCs, rpc)
				commentBuf = nil
				continue
			}
		}

		// Closing brace — exit current block.
		if trimmed == "}" {
			if braceDepth == 0 {
				currentService = nil
				currentMessage = nil
				currentEnum = nil
			}
			commentBuf = nil
			continue
		}

		// Message declaration (top-level only).
		if currentService == nil && currentMessage == nil && currentEnum == nil {
			if m := reMessage.FindStringSubmatch(trimmed); m != nil {
				msg := Message{Name: m[1], Comment: comment}
				mod.Messages = append(mod.Messages, msg)
				currentMessage = &mod.Messages[len(mod.Messages)-1]
				currentService = nil
				currentEnum = nil
				commentBuf = nil
				continue
			}
		}

		// Field declaration (inside message).
		if currentMessage != nil && braceDepth >= 1 {
			if m := reField.FindStringSubmatch(trimmed); m != nil {
				// Collect full field text (options may span multiple lines).
				fullField := line
				for !strings.Contains(fullField, ";") && scanner.Scan() {
					next := scanner.Text()
					fullField += " " + strings.TrimSpace(next)
					braceDepth += strings.Count(next, "{") - strings.Count(next, "}")
				}
				behavior := extractFieldBehavior(fullField)
				field := Field{
					Type:     simplifyType(m[1]),
					Name:     m[2],
					Comment:  comment,
					Required: behavior == "REQUIRED" || behavior == "IDENTIFIER",
					Repeated: strings.HasPrefix(trimmed, "repeated "),
					Behavior: behavior,
				}
				currentMessage.Fields = append(currentMessage.Fields, field)
				commentBuf = nil
				continue
			}
		}

		// Enum declaration (top-level only).
		if currentService == nil && currentMessage == nil && currentEnum == nil {
			if m := reEnum.FindStringSubmatch(trimmed); m != nil {
				en := Enum{Name: m[1], Comment: comment}
				mod.Enums = append(mod.Enums, en)
				currentEnum = &mod.Enums[len(mod.Enums)-1]
				currentService = nil
				currentMessage = nil
				commentBuf = nil
				continue
			}
		}

		// Enum value (inside enum).
		if currentEnum != nil && braceDepth >= 1 {
			if m := reEnumVal.FindStringSubmatch(trimmed); m != nil {
				val := EnumValue{Name: m[1], Number: m[2], Comment: comment}
				currentEnum.Values = append(currentEnum.Values, val)
				commentBuf = nil
				continue
			}
		}

		// Non-matching line — clear comment buffer.
		commentBuf = nil
	}

	return scanner.Err()
}

// generateReadme writes a README.md to the module directory.
// Always overwrites any existing README.md.
func generateReadme(mod Module) (err error) {
	path := filepath.Join(mod.Dir, "README.md")
	absPath, _ := filepath.Abs(path)
	f, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	w := bufio.NewWriter(f)

	fmt.Fprintf(w, "# %s Service\n\n", mod.Name)
	// Kept as a bold paragraph (not a blockquote) so it does not sit adjacent to
	// the [!IMPORTANT] blockquote below — two blockquotes separated by a blank
	// line would trip markdownlint MD028 (blank line inside blockquote).
	fmt.Fprintf(w, "**Package:** `%s`\n\n", mod.Package)
	fmt.Fprintf(w, "> [!IMPORTANT]\n")
	fmt.Fprintf(w, "> Auto-generated by `protodoc`. Do not edit manually.\n\n")

	// Services.
	if len(mod.Services) > 0 {
		fmt.Fprintf(w, "## Services\n\n")
		for _, svc := range mod.Services {
			fmt.Fprintf(w, "### %s\n\n", svc.Name)
			if svc.Comment != "" {
				fmt.Fprintf(w, "%s\n\n", svc.Comment)
			}
			if len(svc.RPCs) > 0 {
				mdTableHeader(w, "Method", "Request", "Response", "Description")
				for _, rpc := range svc.RPCs {
					stream := ""
					if rpc.ServerStream {
						stream = "stream "
					}
					fmt.Fprintf(w, "| `%s` | `%s` | `%s%s` | %s |\n",
						rpc.Name, rpc.Request, stream, rpc.Response, rpc.Comment)
				}
				fmt.Fprintf(w, "\n")
			}
		}
	}

	// Messages.
	if len(mod.Messages) > 0 {
		fmt.Fprintf(w, "## Messages\n\n")
		for _, msg := range mod.Messages {
			fmt.Fprintf(w, "### %s\n\n", msg.Name)
			if msg.Comment != "" {
				fmt.Fprintf(w, "%s\n\n", msg.Comment)
			}
			if len(msg.Fields) > 0 {
				mdTableHeader(w, "Field", "Type", "Behavior", "Description")
				for _, field := range msg.Fields {
					beh := field.Behavior
					if beh == "" && field.Required {
						beh = "REQUIRED"
					}
					if beh != "" {
						beh = "`" + beh + "`"
					} else {
						beh = "-"
					}
					typ := field.Type
					if field.Repeated {
						typ = "repeated " + typ
					}
					fmt.Fprintf(w, "| `%s` | `%s` | %s | %s |\n",
						field.Name, typ, beh, field.Comment)
				}
				fmt.Fprintf(w, "\n")
			}
		}
	}

	// Enums.
	if len(mod.Enums) > 0 {
		fmt.Fprintf(w, "## Enums\n\n")
		for _, en := range mod.Enums {
			fmt.Fprintf(w, "### %s\n\n", en.Name)
			if en.Comment != "" {
				fmt.Fprintf(w, "%s\n\n", en.Comment)
			}
			if len(en.Values) > 0 {
				mdTableHeader(w, "Value", "Number", "Description")
				for _, val := range en.Values {
					fmt.Fprintf(w, "| `%s` | %s | %s |\n", val.Name, val.Number, val.Comment)
				}
				fmt.Fprintf(w, "\n")
			}
		}
	}

	fmt.Fprintf(w, "---\n\n")
	fmt.Fprintf(w, "© 2026 oh-tarnished | Apache 2.0 License\n")

	// bufio.Writer accumulates a sticky error across all the writes above;
	// Flush surfaces it as the single write-error checkpoint.
	return w.Flush()
}

// --- Helpers ---

func titleCase(s string) string {
	words := strings.Split(s, "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func simplifyType(t string) string {
	// Strip package prefixes for readability.
	parts := strings.Split(t, ".")
	return parts[len(parts)-1]
}

// extractFieldBehavior finds (google.api.field_behavior) = X in the field text.
func extractFieldBehavior(fullField string) string {
	matches := reFieldBehavior.FindAllStringSubmatch(fullField, -1)
	if len(matches) == 0 {
		return ""
	}
	for _, m := range matches {
		if len(m) >= 2 && (m[1] == "REQUIRED" || m[1] == "IDENTIFIER") {
			return m[1]
		}
	}
	return matches[0][1]
}
