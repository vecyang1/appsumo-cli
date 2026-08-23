package cli_test

// A gate for credential-shaped literals in tracked documentation.
//
// This repository's existing secret handling protects *values the code touches*:
// cookies are never printed, exports are redacted, and the cookie header only
// goes to appsumo.com. None of that looks at prose. A worked example pasted into
// a Markdown file is documentation, so it passes every one of those checks.
//
// It happened here. Commit 0fe39d2 documented the macOS cookie-decryption helper
// and included the real output of
// `security find-generic-password -s "Chrome Safe Storage"` as the example
// value — a key that decrypts every cookie in the machine's Chrome profile, not
// just AppSumo's — in a repository whose remote is public. It was caught by
// reading the diff, which is exactly the review step that does not scale.
//
// The three-part test for hardening a rule into a gate is met: the failure is
// silent (nothing in the toolchain reads prose), the cost repeats (every
// documentation change is another chance), and the predicate is decidable (a
// long unbroken base64 or hex run inside backticks is a literal, and a
// placeholder is not).
//
// Judgement stays out of it. This does not try to decide whether a value is
// really live; it asserts that examples are written as placeholders.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/vecyang1/appsumo-cli/internal/cli"
)

// backtickedLiteral matches a backticked run of 20+ characters drawn only from
// the base64 alphabet. Identifier separators are deliberately excluded: an
// underscore or hyphen is what an API field name has and a key does not, so
// `quantity_remaining_below_threshold` never becomes a candidate at all.
var backtickedLiteral = regexp.MustCompile("`([A-Za-z0-9+/=]{20,})`")

// allowedLiterals are non-secret long tokens this repository legitimately
// documents. Each is an exact string, never a pattern, so widening the allowance
// is a visible edit rather than an accident.
var allowedLiterals = map[string]string{}

// placeholderHints mark a value the author already wrote as an example rather
// than a real one.
var placeholderHints = []string{
	"example", "placeholder", "your", "redacted", "xxx", "changeme",
	"fixture", "dummy", "sample", "fake",
}

func TestTrackedMarkdownHasNoCredentialShapedLiterals(t *testing.T) {
	root := repoRoot(t)
	files := trackedMarkdown(t, root)
	if len(files) == 0 {
		t.Fatal("found no Markdown files to grade; the selector is broken")
	}

	graded := 0
	for _, file := range files {
		body, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		graded++
		for lineNumber, line := range strings.Split(string(body), "\n") {
			for _, match := range backtickedLiteral.FindAllStringSubmatch(line, -1) {
				literal := match[1]
				if _, allowed := allowedLiterals[literal]; allowed {
					continue
				}
				if looksLikePlaceholder(literal) || !isCredentialShaped(literal) {
					continue
				}
				t.Errorf("%s:%d documents a credential-shaped literal %q\n"+
					"  replace it with an obvious placeholder such as `<value-from-your-keychain>`\n"+
					"  or add it to allowedLiterals with the reason it is not a secret",
					file, lineNumber+1, redactForReport(literal))
			}
		}
	}
	t.Logf("graded %d tracked Markdown files for credential-shaped literals", graded)
}

func looksLikePlaceholder(literal string) bool {
	lowered := strings.ToLower(literal)
	for _, hint := range placeholderHints {
		if strings.Contains(lowered, hint) {
			return true
		}
	}
	return false
}

// isCredentialShaped decides whether a long base64-alphabet run reads like an
// encoded value rather than like something a person wrote.
//
// The distinguishing property is character-class mixing without word breaks. A
// URL path has slashes but no digits or capitals; a Go identifier has capitals
// but no digits; a base64 key has all three and no separators. Both real
// false positives this gate produced on its first run — `/api/sessions/current/`
// and `TestFetchAllReviewsStopsWhenOffsetIsIgnored` — fail here for those
// reasons, and neither needed an allowlist entry.
func isCredentialShaped(literal string) bool {
	if strings.HasPrefix(literal, "/") || strings.Contains(literal, "//") {
		return false
	}
	// Base64 padding is decisive on its own, and it is the one property that
	// does not depend on luck. A 16-byte key encodes to 24 characters ending
	// "==" whatever its bytes happen to be, whereas the mixed-class test below
	// misses roughly one such key in twenty — the ones that contain no digit.
	// That gap was found by probing this gate with a synthetic key and watching
	// it pass.
	if strings.HasSuffix(literal, "=") {
		return true
	}
	var hasDigit, hasUpper, hasLower bool
	for _, char := range literal {
		switch {
		case char >= '0' && char <= '9':
			hasDigit = true
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		}
	}
	if hasDigit && hasUpper && hasLower {
		return true
	}
	// A long single-case hex run is a hash or token even without mixed case.
	return len(literal) >= 32 && hasDigit && (hasLower != hasUpper)
}

// redactForReport names the problem without reprinting the value. A gate that
// echoes the secret it found puts it into the CI log, which is the one place
// nobody thinks to scrub.
func redactForReport(literal string) string {
	return fmt.Sprintf("%d-character literal starting %q", len(literal), literal[:4])
}

// TestCredentialShapeVectors pins the predicate against fixed inputs rather than
// against whatever happens to be in the docs today.
//
// The false-negative row is the reason this exists: probing the gate with a
// randomly generated key passed once and failed on the next attempt, because a
// 16-byte key contains no digit about one time in twenty. A gate verified by a
// coin flip is verified on the flips that came up heads.
func TestCredentialShapeVectors(t *testing.T) {
	credentials := []string{
		"7IdItnVNcrqDYmx+C4K8aw==",                 // 16-byte key, mixed classes
		"QRSTUVWXYZabcdefghijkl==",                 // 16-byte key shape with no digit at all
		"ghp0123456789abcdefABCDEF0123456789",      // long mixed token
		"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0", // 40-char hex digest
	}
	for _, literal := range credentials {
		if !isCredentialShaped(literal) {
			t.Errorf("credential-shaped literal was not recognised: %s", redactForReport(literal))
		}
	}

	notCredentials := []string{
		"/api/sessions/current/",                      // URL path
		"TestFetchAllReviewsStopsWhenOffsetIsIgnored", // Go identifier
		"getStaticPropsRenderedPage",                  // camelCase prose
		"AppSumoCommandLineInterface",                 // TitleCase prose
	}
	for _, literal := range notCredentials {
		if isCredentialShaped(literal) {
			t.Errorf("ordinary documentation text was flagged as a credential: %q", literal)
		}
	}
}

// TestErrorRemediesNameRealFlags is the decidable slice of "read your error
// strings as the user": if a message tells someone to pass a flag, the parser
// has to define that flag. Wording and tone stay in review; a remedy that
// cannot be followed does not.
func TestErrorRemediesNameRealFlags(t *testing.T) {
	root := cli.NewRoot(cli.Options{Out: io.Discard, Err: io.Discard})
	flagPattern := regexp.MustCompile(`--[a-z][a-z0-9-]+`)

	graded := 0
	for _, file := range goSources(t, repoRoot(t)) {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, message := range errorMessageLiterals(string(body)) {
			for _, flag := range flagPattern.FindAllString(message, -1) {
				graded++
				name := strings.TrimPrefix(flag, "--")
				if root.PersistentFlags().Lookup(name) != nil {
					continue
				}
				if commandDefinesFlag(root, name) {
					continue
				}
				t.Errorf("%s raises an error naming %s, which no command defines", file, flag)
			}
		}
	}
	t.Logf("graded %d flag references inside error messages", graded)
}

// errorMessageLiterals pulls the format strings out of fmt.Errorf calls.
var errorfPattern = regexp.MustCompile(`fmt\.Errorf\(\s*"((?:[^"\\]|\\.)*)"`)

func errorMessageLiterals(source string) []string {
	var messages []string
	for _, match := range errorfPattern.FindAllStringSubmatch(source, -1) {
		messages = append(messages, match[1])
	}
	// Remedies are sometimes built as plain strings rather than at the raise
	// site; catch the ones that read like instructions.
	for _, line := range strings.Split(source, "\n") {
		if strings.Contains(line, "pass --") || strings.Contains(line, "run `appsumo") {
			messages = append(messages, line)
		}
	}
	return messages
}

func commandDefinesFlag(root *cobra.Command, name string) bool {
	found := false
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Flags().Lookup(name) != nil || cmd.PersistentFlags().Lookup(name) != nil {
			found = true
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	return found
}

func goSources(t *testing.T, root string) []string {
	t.Helper()
	command := exec.Command("git", "ls-files", "*.go")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git ls-files *.go: %v", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(root, line))
	}
	return files
}
