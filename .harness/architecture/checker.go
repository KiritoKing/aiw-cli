package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/OpenPeeDeeP/depguard/v2"
	"github.com/gobwas/glob"
	"golang.org/x/tools/go/analysis"
)

const rulesRelativePath = ".harness/architecture/rules.json"

type rulesFile struct {
	Version int              `json:"version"`
	Layers  map[string]layer `json:"layers"`
}

type layer struct {
	Files []string `json:"files"`
	Allow []string `json:"allow"`
}

type fileRule struct {
	negate  bool
	matcher glob.Glob
}

type compiledLayer struct {
	name  string
	files []fileRule
}

type checker struct {
	analyzer *analysis.Analyzer
	layers   []compiledLayer
	count    int
}

type sourceFile struct {
	absolute string
	relative string
}

type result struct {
	files  int
	layers int
}

func Check(root string) (result, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return result{}, fmt.Errorf("resolve repository path: %w", err)
	}
	absoluteRoot, err = filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return result{}, fmt.Errorf("resolve repository path: %w", err)
	}
	rawRules, err := os.ReadFile(filepath.Join(absoluteRoot, filepath.FromSlash(rulesRelativePath)))
	if err != nil {
		return result{}, fmt.Errorf("read %s: %w", rulesRelativePath, err)
	}
	return checkRepository(absoluteRoot, rawRules)
}

func checkRepository(root string, rawRules []byte) (result, error) {
	c, err := newChecker(rawRules)
	if err != nil {
		return result{}, err
	}
	files, err := collectGoFiles(root)
	if err != nil {
		return result{}, err
	}
	for _, file := range files {
		matches := c.matchingLayers(file.relative)
		if len(matches) != 1 {
			return result{}, fmt.Errorf("Go file %s matches %d architecture layers: %s", file.relative, len(matches), strings.Join(matches, ", "))
		}
	}

	fset := token.NewFileSet()
	astFiles := make([]*ast.File, 0, len(files))
	for _, file := range files {
		source, err := os.ReadFile(file.absolute)
		if err != nil {
			return result{}, fmt.Errorf("read Go file %s: %w", file.relative, err)
		}
		parsed, err := parser.ParseFile(fset, file.relative, source, parser.ParseComments|parser.AllErrors)
		if err != nil {
			return result{}, fmt.Errorf("parse Go file %s: %w", file.relative, err)
		}
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return result{}, fmt.Errorf("decode import in %s: %w", file.relative, err)
			}
			spec.Path.Value = strconv.Quote(importPath)
		}
		astFiles = append(astFiles, parsed)
	}

	var diagnostics []analysis.Diagnostic
	_, err = c.analyzer.Run(&analysis.Pass{
		Fset:  fset,
		Files: astFiles,
		Report: func(diagnostic analysis.Diagnostic) {
			diagnostics = append(diagnostics, diagnostic)
		},
	})
	if err != nil {
		return result{}, fmt.Errorf("run depguard analyzer: %w", err)
	}
	if len(diagnostics) > 0 {
		sort.Slice(diagnostics, func(i, j int) bool {
			left, right := fset.Position(diagnostics[i].Pos), fset.Position(diagnostics[j].Pos)
			if left.Filename != right.Filename {
				return left.Filename < right.Filename
			}
			if left.Line != right.Line {
				return left.Line < right.Line
			}
			if left.Column != right.Column {
				return left.Column < right.Column
			}
			return diagnostics[i].Message < diagnostics[j].Message
		})
		lines := make([]string, 0, len(diagnostics))
		for _, diagnostic := range diagnostics {
			position := fset.Position(diagnostic.Pos)
			lines = append(lines, fmt.Sprintf("%s:%d:%d: %s", position.Filename, position.Line, position.Column, diagnostic.Message))
		}
		return result{}, fmt.Errorf("architecture violations:\n%s", strings.Join(lines, "\n"))
	}
	return result{files: len(files), layers: c.count}, nil
}

func newChecker(data []byte) (*checker, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, fmt.Errorf("invalid architecture rules JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config rulesFile
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode architecture rules: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("architecture rules contain trailing JSON")
		}
		return nil, fmt.Errorf("decode trailing architecture rules data: %w", err)
	}
	if config.Version != 1 {
		return nil, fmt.Errorf("unsupported architecture rules version %d", config.Version)
	}
	if len(config.Layers) == 0 {
		return nil, errors.New("architecture rules must define at least one layer")
	}

	standard, err := standardLibraryPrefixes()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(config.Layers))
	for name := range config.Layers {
		names = append(names, name)
	}
	sort.Strings(names)
	settings := make(depguard.LinterSettings, len(names))
	compiled := make([]compiledLayer, 0, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name {
			return nil, fmt.Errorf("architecture layer name %q is empty or has surrounding whitespace", name)
		}
		rule := config.Layers[name]
		if len(rule.Files) == 0 {
			return nil, fmt.Errorf("architecture layer %q must define file globs", name)
		}
		if len(rule.Allow) == 0 {
			return nil, fmt.Errorf("architecture layer %q must define an allowlist", name)
		}

		files := make([]fileRule, 0, len(rule.Files))
		seenFiles := make(map[string]struct{}, len(rule.Files))
		positiveFiles := 0
		depguardFiles := make([]string, 0, len(rule.Files))
		for _, pattern := range rule.Files {
			compiledFile, err := compileFileRule(pattern)
			if err != nil {
				return nil, fmt.Errorf("architecture layer %q: %w", name, err)
			}
			if _, duplicate := seenFiles[pattern]; duplicate {
				return nil, fmt.Errorf("architecture layer %q repeats file glob %q", name, pattern)
			}
			seenFiles[pattern] = struct{}{}
			if !compiledFile.negate {
				positiveFiles++
			}
			files = append(files, compiledFile)
			depguardFiles = append(depguardFiles, pattern)
		}
		if positiveFiles == 0 {
			return nil, fmt.Errorf("architecture layer %q needs a positive file glob", name)
		}

		allow := make([]string, 0, len(rule.Allow)+len(standard))
		seenAllow := make(map[string]struct{}, len(rule.Allow))
		standardCount := 0
		for _, item := range rule.Allow {
			if _, duplicate := seenAllow[item]; duplicate {
				return nil, fmt.Errorf("architecture layer %q repeats allowlist entry %q", name, item)
			}
			seenAllow[item] = struct{}{}
			if item == "$gostd" {
				standardCount++
				allow = append(allow, standard...)
				continue
			}
			if err := validateImportAllow(item); err != nil {
				return nil, fmt.Errorf("architecture layer %q: %w", name, err)
			}
			allow = append(allow, item)
		}
		if standardCount != 1 {
			return nil, fmt.Errorf("architecture layer %q must contain exactly one $gostd entry", name)
		}
		settings[name] = &depguard.List{ListMode: "Strict", Files: depguardFiles, Allow: allow}
		compiled = append(compiled, compiledLayer{name: name, files: files})
	}
	analyzer, err := depguard.NewAnalyzer(&settings)
	if err != nil {
		return nil, fmt.Errorf("compile depguard rules: %w", err)
	}
	return &checker{analyzer: analyzer, layers: compiled, count: len(names)}, nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("trailing JSON value")
}

func consumeJSONValue(decoder *json.Decoder) error {
	tokenValue, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := tokenValue.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return errors.New("unterminated JSON object")
		}
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return errors.New("unterminated JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}

func compileFileRule(pattern string) (fileRule, error) {
	if pattern == "" || strings.TrimSpace(pattern) != pattern || strings.Contains(pattern, "\\") {
		return fileRule{}, fmt.Errorf("invalid file glob %q", pattern)
	}
	negate := strings.HasPrefix(pattern, "!")
	value := strings.TrimPrefix(pattern, "!")
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "$") {
		return fileRule{}, fmt.Errorf("invalid file glob %q", pattern)
	}
	separator := strings.LastIndex(value, "/")
	if separator <= 0 || separator == len(value)-1 {
		return fileRule{}, fmt.Errorf("file glob %q must target a direct package directory", pattern)
	}
	directory, filename := value[:separator], value[separator+1:]
	if strings.ContainsAny(directory, "*?[]{}") || !strings.HasSuffix(filename, ".go") {
		return fileRule{}, fmt.Errorf("file glob %q must use a literal directory and Go filename", pattern)
	}
	for _, segment := range strings.Split(directory, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fileRule{}, fmt.Errorf("file glob %q has an invalid directory", pattern)
		}
	}
	matcher, err := glob.Compile(value, '/')
	if err != nil {
		return fileRule{}, fmt.Errorf("invalid file glob %q: %w", pattern, err)
	}
	return fileRule{negate: negate, matcher: matcher}, nil
}

func validateImportAllow(value string) error {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "*?!") || strings.Contains(value, "\\") {
		return fmt.Errorf("invalid import allowlist entry %q", value)
	}
	if strings.Contains(value, "$gostd") || strings.Contains(value, "$") && !strings.HasSuffix(value, "$") {
		return fmt.Errorf("invalid import allowlist entry %q", value)
	}
	if !strings.HasSuffix(value, "$") && !strings.HasSuffix(value, "/") {
		return fmt.Errorf("import allowlist entry %q must end in $ or /", value)
	}
	if value == "$" || value == "/" {
		return fmt.Errorf("invalid import allowlist entry %q", value)
	}
	return nil
}

func standardLibraryPrefixes() ([]string, error) {
	goRoot := os.Getenv("GOROOT")
	if goRoot == "" {
		goRoot = runtime.GOROOT()
	}
	if goRoot == "" {
		return nil, errors.New("cannot locate GOROOT to expand $gostd")
	}
	entries, err := os.ReadDir(filepath.Join(goRoot, "src"))
	if err != nil {
		return nil, fmt.Errorf("read GOROOT/src for $gostd: %w", err)
	}
	allow := make([]string, 0, len(entries)*2)
	for _, entry := range entries {
		if entry.IsDir() {
			allow = append(allow, entry.Name()+"$", entry.Name()+"/")
		}
	}
	sort.Strings(allow)
	return allow, nil
}

func (c *checker) matchingLayers(filename string) []string {
	var matches []string
	for _, layer := range c.layers {
		included := false
		excluded := false
		for _, rule := range layer.files {
			if rule.matcher.Match(filename) {
				if rule.negate {
					excluded = true
				} else {
					included = true
				}
			}
		}
		if included && !excluded {
			matches = append(matches, layer.name)
		}
	}
	return matches
}

func collectGoFiles(root string) ([]sourceFile, error) {
	var files []sourceFile
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("scan repository: %w", walkErr)
		}
		if filename == root {
			return nil
		}
		relative, err := filepath.Rel(root, filename)
		if err != nil {
			return fmt.Errorf("make repository-relative path: %w", err)
		}
		slashPath := filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return checkSymlink(filename, slashPath)
		}
		if entry.IsDir() {
			if excludedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(slashPath, ".go") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect Go file %s: %w", slashPath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Go source %s is not a regular file", slashPath)
		}
		files = append(files, sourceFile{absolute: filename, relative: slashPath})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].relative < files[j].relative })
	return files, nil
}

func checkSymlink(filename, relative string) error {
	target, statErr := os.Stat(filename)
	goSource := strings.HasSuffix(relative, ".go")
	resolved, resolveErr := filepath.EvalSymlinks(filename)
	if resolveErr == nil && strings.HasSuffix(filepath.Base(resolved), ".go") {
		goSource = true
	}
	if goSource {
		return fmt.Errorf("Go source symlink is forbidden: %s", relative)
	}
	if !insideCodeRoot(relative) {
		return nil
	}
	if isCodeRoot(relative) || statErr != nil || target.IsDir() {
		return fmt.Errorf("source directory symlink is forbidden: %s", relative)
	}
	return nil
}

func insideCodeRoot(relative string) bool {
	first, _, _ := strings.Cut(relative, "/")
	switch first {
	case "cmd", "internal", "skills", ".harness":
		return true
	default:
		return false
	}
}

func isCodeRoot(relative string) bool {
	switch relative {
	case "cmd", "internal", "skills", ".harness":
		return true
	default:
		return false
	}
}

func excludedDirectory(name string) bool {
	switch name {
	case ".git", "vendor", "node_modules":
		return true
	default:
		return false
	}
}
