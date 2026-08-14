package cli_test

// Database isolation for the whole CLI test binary.
//
// Every command that touches SQLite resolves its path as: --db flag, then
// APPSUMO_DB_PATH, then the user's config directory. Passing DBPath in each test
// works, and is exactly the kind of isolation that holds until somebody adds a
// test and forgets — at which point the suite writes into the developer's real
// account database instead of failing.
//
// So the sandbox is set once here, structurally, for the process. A test that
// forgets DBPath still lands in a temp directory.
//
// The backstop that asserts the real database was untouched runs after m.Run(),
// not as a Test function. Go runs test files in alphabetical order, and this
// file sorts before every file that writes — so as an ordinary test it would
// have taken its "after" reading before any writer had run, and passed no
// matter what the suite did. A guard that cannot observe the thing it guards is
// worse than no guard, because it reads as coverage.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	sandbox, err := os.MkdirTemp("", "appsumo-cli-test-db-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create test sandbox: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("APPSUMO_DB_PATH", filepath.Join(sandbox, "appsumo.db")); err != nil {
		fmt.Fprintf(os.Stderr, "set sandbox database path: %v\n", err)
		os.Exit(1)
	}

	before := describeRealDatabase()
	code := m.Run()
	after := describeRealDatabase()
	_ = os.RemoveAll(sandbox)

	if before != after {
		fmt.Fprintf(os.Stderr,
			"\nFAIL: the test suite modified the real account database\n  before: %s\n  after:  %s\n"+
				"  a test is resolving the default database path instead of the sandbox\n", before, after)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// TestSandboxDatabasePathIsSet asserts the cause rather than the symptom. It
// fails immediately if the sandbox was never installed, which the after-run
// comparison would only reveal once something had already been written.
func TestSandboxDatabasePathIsSet(t *testing.T) {
	sandbox := os.Getenv("APPSUMO_DB_PATH")
	if sandbox == "" {
		t.Fatal("APPSUMO_DB_PATH is empty; the test binary has no database sandbox")
	}
	tempRoot, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatalf("resolve temp directory: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Dir(sandbox))
	if err != nil {
		t.Fatalf("resolve sandbox directory: %v", err)
	}
	if !filepath.HasPrefix(resolved, tempRoot) {
		t.Fatalf("sandbox database %s is not under the temp directory %s", resolved, tempRoot)
	}
}

// describeRealDatabase reports size and modification time, or that the file is
// absent. Absent is a state worth comparing: a suite that creates the real
// database where none existed has also escaped the sandbox.
func describeRealDatabase() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "unknown:" + err.Error()
	}
	path := filepath.Join(configDir, "appsumo-cli", "appsumo.db")
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "absent"
	}
	if err != nil {
		return "unreadable:" + err.Error()
	}
	return fmt.Sprintf("size=%d mtime=%s", info.Size(), info.ModTime().UTC().Format("2006-01-02T15:04:05.000000000Z"))
}
