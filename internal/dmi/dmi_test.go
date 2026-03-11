package dmi

import (
	"io/fs"
	"os"
	"testing"
)

type mockEntry struct {
	name  string
	isDir bool
}

func (e mockEntry) Name() string { return e.name }
func (e mockEntry) IsDir() bool  { return e.isDir }
func (e mockEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}
func (e mockEntry) Info() (fs.FileInfo, error) { return nil, nil }

type mockReader struct {
	dirs  map[string][]mockEntry
	files map[string]string
}

func (m *mockReader) ReadDir(name string) ([]fs.DirEntry, error) {
	ents, ok := m.dirs[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	out := make([]fs.DirEntry, len(ents))
	for i, e := range ents {
		out[i] = e
	}
	return out, nil
}

func (m *mockReader) ReadFile(name string) ([]byte, error) {
	s, ok := m.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return []byte(s), nil
}

// ---------------------------------------------------------------------------
// Load tests
// ---------------------------------------------------------------------------

// TestLoad_ReadsAllNonDirEntries verifies that all non-directory entries are
// read and their values are trimmed of whitespace.
func TestLoad_ReadsAllNonDirEntries(t *testing.T) {
	r := &mockReader{
		dirs: map[string][]mockEntry{
			"/sys/class/dmi/id": {
				{name: "board_vendor"},
				{name: "chassis_serial"},
				{name: "product_name"},
			},
		},
		files: map[string]string{
			"/sys/class/dmi/id/board_vendor":   "  Dell Inc.  \n",
			"/sys/class/dmi/id/chassis_serial": "ABC123\n",
			"/sys/class/dmi/id/product_name":   "PowerEdge R740\n",
		},
	}

	info, err := Load(r, "/sys/class/dmi/id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cases := map[string]string{
		"board_vendor":   "Dell Inc.",
		"chassis_serial": "ABC123",
		"product_name":   "PowerEdge R740",
	}
	for key, want := range cases {
		got, err := info.Get(key)
		if err != nil {
			t.Errorf("Get(%q): unexpected error: %v", key, err)
			continue
		}
		if got != want {
			t.Errorf("Get(%q): want %q, got %q", key, want, got)
		}
	}
}

// TestLoad_SkipsEntriesWhereReadFileFails verifies best-effort behaviour: when
// ReadFile fails for an entry, the entry is silently skipped and Load still
// succeeds.
func TestLoad_SkipsEntriesWhereReadFileFails(t *testing.T) {
	r := &mockReader{
		dirs: map[string][]mockEntry{
			"/sys/class/dmi/id": {
				{name: "chassis_serial"},
				{name: "root_only_field"}, // ReadFile will fail for this one
			},
		},
		files: map[string]string{
			"/sys/class/dmi/id/chassis_serial": "XYZ789",
			// root_only_field intentionally absent from files map
		},
	}

	info, err := Load(r, "/sys/class/dmi/id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// chassis_serial should still be available
	got, err := info.Get("chassis_serial")
	if err != nil {
		t.Fatalf("Get(chassis_serial): unexpected error: %v", err)
	}
	if got != "XYZ789" {
		t.Errorf("Get(chassis_serial): want 'XYZ789', got %q", got)
	}

	// root_only_field should not be present
	_, err = info.Get("root_only_field")
	if err == nil {
		t.Error("expected error for missing key 'root_only_field', got nil")
	}
}

// TestLoad_SkipsDirectoryEntries verifies that directory entries in the basePath
// are not read or added to the info map.
func TestLoad_SkipsDirectoryEntries(t *testing.T) {
	r := &mockReader{
		dirs: map[string][]mockEntry{
			"/sys/class/dmi/id": {
				{name: "chassis_serial", isDir: false},
				{name: "power", isDir: true}, // directory — should be skipped
			},
		},
		files: map[string]string{
			"/sys/class/dmi/id/chassis_serial": "SERIAL001",
			// "power" is not in files; if Load tries to read it the mock returns
			// ErrNotExist, but we want to confirm it is never attempted.
			// We omit it so that if the code inadvertently tries to read a dir
			// entry and succeeds (hypothetically), the test would still fail via
			// the Get check below.
		},
	}

	info, err := Load(r, "/sys/class/dmi/id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "power" should not be in the data map
	_, err = info.Get("power")
	if err == nil {
		t.Error("expected error for directory entry 'power', but Get succeeded")
	}
}

// TestLoad_ReturnsErrorWhenReadDirFails verifies that Load propagates a ReadDir
// error.
func TestLoad_ReturnsErrorWhenReadDirFails(t *testing.T) {
	r := &mockReader{
		dirs:  map[string][]mockEntry{}, // basePath not present → ErrNotExist
		files: map[string]string{},
	}

	_, err := Load(r, "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error from Load when ReadDir fails, got nil")
	}
}

// TestLoad_DefaultBasePath verifies that when basePath is empty the default
// "/sys/class/dmi/id" is used, evidenced by ReadDir being called with that
// path (the mock only has that key defined).
func TestLoad_DefaultBasePath(t *testing.T) {
	r := &mockReader{
		dirs: map[string][]mockEntry{
			"/sys/class/dmi/id": {
				{name: "chassis_serial"},
			},
		},
		files: map[string]string{
			"/sys/class/dmi/id/chassis_serial": "DEFAULTPATH",
		},
	}

	// Pass empty basePath — Load must fall back to /sys/class/dmi/id
	info, err := Load(r, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := info.Get("chassis_serial")
	if err != nil {
		t.Fatalf("Get(chassis_serial): unexpected error: %v", err)
	}
	if got != "DEFAULTPATH" {
		t.Errorf("expected 'DEFAULTPATH', got %q", got)
	}
}

// TestLoad_BasepathStoredInInfo verifies that the resolved basePath is stored
// on the returned Info struct.
func TestLoad_BasepathStoredInInfo(t *testing.T) {
	r := &mockReader{
		dirs: map[string][]mockEntry{
			"/sys/class/dmi/id": {},
		},
		files: map[string]string{},
	}

	info, err := Load(r, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.BasePath != "/sys/class/dmi/id" {
		t.Errorf("BasePath: want '/sys/class/dmi/id', got %q", info.BasePath)
	}
}

// ---------------------------------------------------------------------------
// Info.Get tests
// ---------------------------------------------------------------------------

// TestGet_ReturnsValueAndNilForExistingKey verifies the happy path.
func TestGet_ReturnsValueAndNilForExistingKey(t *testing.T) {
	r := &mockReader{
		dirs: map[string][]mockEntry{
			"/sys/class/dmi/id": {{name: "product_uuid"}},
		},
		files: map[string]string{
			"/sys/class/dmi/id/product_uuid": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		},
	}
	info, err := Load(r, "/sys/class/dmi/id")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	got, err := info.Get("product_uuid")
	if err != nil {
		t.Fatalf("Get(product_uuid): unexpected error: %v", err)
	}
	if got != "6ba7b810-9dad-11d1-80b4-00c04fd430c8" {
		t.Errorf("Get: want UUID value, got %q", got)
	}
}

// TestGet_ReturnsErrorForMissingKey verifies that Get returns a non-nil error
// when the key was never loaded.
func TestGet_ReturnsErrorForMissingKey(t *testing.T) {
	r := &mockReader{
		dirs:  map[string][]mockEntry{"/sys/class/dmi/id": {}},
		files: map[string]string{},
	}
	info, err := Load(r, "/sys/class/dmi/id")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = info.Get("does_not_exist")
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

// ---------------------------------------------------------------------------
// Info.ChassisSerial tests
// ---------------------------------------------------------------------------

// TestChassisSerial_ReturnsValue verifies the happy path.
func TestChassisSerial_ReturnsValue(t *testing.T) {
	r := &mockReader{
		dirs: map[string][]mockEntry{
			"/sys/class/dmi/id": {{name: "chassis_serial"}},
		},
		files: map[string]string{
			"/sys/class/dmi/id/chassis_serial": "CHASSIS001\n",
		},
	}
	info, err := Load(r, "/sys/class/dmi/id")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	got, err := info.ChassisSerial()
	if err != nil {
		t.Fatalf("ChassisSerial: unexpected error: %v", err)
	}
	if got != "CHASSIS001" {
		t.Errorf("ChassisSerial: want 'CHASSIS001', got %q", got)
	}
}

// TestChassisSerial_ReturnsErrorWhenAbsent verifies that ChassisSerial returns
// an error when the "chassis_serial" key was not loaded.
func TestChassisSerial_ReturnsErrorWhenAbsent(t *testing.T) {
	r := &mockReader{
		dirs:  map[string][]mockEntry{"/sys/class/dmi/id": {}},
		files: map[string]string{},
	}
	info, err := Load(r, "/sys/class/dmi/id")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = info.ChassisSerial()
	if err == nil {
		t.Error("expected error from ChassisSerial when key absent, got nil")
	}
}
