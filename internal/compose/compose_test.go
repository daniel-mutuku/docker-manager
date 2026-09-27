package compose

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{"a", "my-app", "my_app_2", "app123"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"", "-leading-dash", "Uppercase", "has space", "../../etc",
		"has/slash", "trailing.", "with.dot",
	}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want an error", name)
		}
	}
}

func TestDeployer_Up_RejectsInvalidNameWithoutRunning(t *testing.T) {
	ran := false
	d := &Deployer{baseDir: t.TempDir(), run: func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		ran = true
		return nil, nil
	}}

	_, err := d.Up(context.Background(), "Not Valid", "services: {}")
	if err == nil {
		t.Fatal("expected an error for an invalid stack name")
	}
	if ran {
		t.Error("the underlying command should never run for an invalid name")
	}
}

func TestDeployer_Up_WritesComposeFileAndRunsExpectedCommand(t *testing.T) {
	baseDir := t.TempDir()
	var gotDir, gotName string
	var gotArgs []string

	d := &Deployer{baseDir: baseDir, run: func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		gotDir, gotName, gotArgs = dir, name, args
		return []byte("up output"), nil
	}}

	content := "services:\n  web:\n    image: nginx\n"
	out, err := d.Up(context.Background(), "my-app", content)
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if string(out) != "up output" {
		t.Errorf("output = %q, want %q", out, "up output")
	}

	wantDir := filepath.Join(baseDir, "my-app")
	if gotDir != wantDir {
		t.Errorf("run dir = %q, want %q", gotDir, wantDir)
	}
	if gotName != "docker" {
		t.Errorf("run command = %q, want docker", gotName)
	}

	written, err := os.ReadFile(filepath.Join(wantDir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading written compose file: %v", err)
	}
	if string(written) != content {
		t.Errorf("written content = %q, want %q", written, content)
	}

	wantArgs := []string{"compose", "-p", "my-app", "-f", "docker-compose.yml", "up", "-d", "--remove-orphans"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Errorf("args[%d] = %q, want %q", i, gotArgs[i], wantArgs[i])
		}
	}
}

func TestDeployer_Down_ErrorsWhenNeverDeployed(t *testing.T) {
	d := &Deployer{baseDir: t.TempDir(), run: func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		t.Fatal("run should not be called when the compose file doesn't exist")
		return nil, nil
	}}

	_, err := d.Down(context.Background(), "never-deployed")
	if err == nil {
		t.Fatal("expected an error tearing down a stack that was never deployed")
	}
}
