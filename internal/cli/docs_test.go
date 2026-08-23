package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/cli"
)

// commandPattern matches a documented invocation: the word `appsumo` followed by
// a lowercase subcommand. It deliberately skips `./cmd/appsumo` style build
// paths, whose next token starts with a dot or slash.
var commandPattern = regexp.MustCompile(`\bappsumo ([a-z][^` + "`" + `\n]*)`)

// placeholderPattern matches doc placeholders such as <product-slug>.
var placeholderPattern = regexp.MustCompile(`<[^>]+>`)

// TestDocumentedCommandsParse keeps every `appsumo ...` string in the tracked
// Markdown runnable. Documentation is the one interface nothing else asserts on,
// and a flag renamed in code leaves a command in the README that exits before it
// does anything.
//
// The gate reports how many commands it graded: a selector that silently narrows
// later shows up as a number that dropped rather than as continued green.
func TestDocumentedCommandsParse(t *testing.T) {
	root := repoRoot(t)
	files := trackedMarkdown(t, root)
	if len(files) < 10 {
		t.Fatalf("graded only %d markdown files; the doc selector narrowed", len(files))
	}

	graded := 0
	for _, relative := range files {
		content, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		for _, line := range strings.Split(string(content), "\n") {
			for _, match := range commandPattern.FindAllStringSubmatch(line, -1) {
				args := splitDocumentedCommand(match[1])
				if len(args) == 0 {
					continue
				}
				graded++
				if err := parseOnly(args); err != nil {
					t.Errorf("%s documents `appsumo %s`, which does not parse: %v",
						relative, strings.Join(args, " "), err)
				}
			}
		}
	}
	if graded < 8 {
		t.Fatalf("graded only %d documented commands; expected the full command surface", graded)
	}
	t.Logf("graded %d documented commands across %d markdown files", graded, len(files))
}

// splitDocumentedCommand trims shell decoration a doc line carries around the
// command itself, then substitutes placeholders with a parseable token.
func splitDocumentedCommand(raw string) []string {
	for _, cut := range []string{">", "|", "&&", ";", "  #"} {
		if index := strings.Index(raw, cut); index >= 0 {
			raw = raw[:index]
		}
	}
	raw = placeholderPattern.ReplaceAllString(raw, "placeholder")
	raw = strings.Trim(raw, " \t`\"',.")

	var args []string
	for _, field := range strings.Fields(raw) {
		args = append(args, strings.Trim(field, `"'`))
	}
	return args
}

// parseOnly resolves the subcommand and validates flags without running it.
// --help short-circuits before RunE but still rejects unknown flags and
// unknown subcommands.
func parseOnly(args []string) error {
	var out, errOut bytes.Buffer
	cmd := cli.NewRoot(cli.Options{Out: &out, Err: &errOut})
	cmd.SetArgs(append(append([]string{}, args...), "--help"))
	return cmd.Execute()
}

func repoRoot(t *testing.T) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}
	root := filepath.Clean(filepath.Join(working, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %s has no go.mod: %v", root, err)
	}
	return root
}

// trackedMarkdown returns every Markdown file git knows about, tracked or newly
// added but not ignored, as repo-relative paths.
//
// It asks git rather than globbing a hand-written list of directories. The
// earlier selector covered the repository root and docs/ only, which is a
// specific answer to a general question — it graded 9 files and silently skipped
// 6 more, and a document nobody has opened is exactly where a stale command or a
// pasted secret survives.
func trackedMarkdown(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]struct{}{}
	var files []string
	for _, args := range [][]string{
		{"ls-files", "*.md"},
		{"ls-files", "--others", "--exclude-standard", "*.md"},
	} {
		command := exec.Command("git", args...)
		command.Dir = root
		output, err := command.Output()
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if _, duplicate := seen[line]; duplicate {
				continue
			}
			seen[line] = struct{}{}
			files = append(files, line)
		}
	}
	sort.Strings(files)
	return files
}

// runCLICapturingStderr runs a command that is expected to succeed and returns
// both streams, so a test can assert that a warning went to stderr and that
// stdout stayed a clean pipe.
func runCLICapturingStderr(t *testing.T, options cli.Options, args ...string) (string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	options.Out = &out
	options.Err = &errOut
	cmd := cli.NewRoot(options)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("appsumo %s failed: %v\nstderr:\n%s", strings.Join(args, " "), err, errOut.String())
	}
	return out.String(), errOut.String()
}

// runCLIExpectingError runs a command that must fail and returns its error.
func runCLIExpectingError(t *testing.T, options cli.Options, args ...string) error {
	t.Helper()
	var out, errOut bytes.Buffer
	options.Out = &out
	options.Err = &errOut
	cmd := cli.NewRoot(options)
	cmd.SetArgs(args)
	return cmd.Execute()
}
