//go:build ci

package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type Config struct {
	TempOutputDir     string
	CompilerSrcDir    string
	CompilerTargetDir string
	ScarbSrcDir       string
	ScarbTargetDir    string
}

func loadConfig() Config {
	return Config{
		TempOutputDir:     "/tmp/juno-cairo-artifacts",
		CompilerSrcDir:    "../compiler/src",
		CompilerTargetDir: "../compiler/target",
		ScarbSrcDir:       "../scarb/src",
		ScarbTargetDir:    "../scarb/target/dev",
	}
}

func compileWithCairoCompiler(cfg Config) error {
	files, err := filepath.Glob(filepath.Join(cfg.CompilerSrcDir, "*.cairo"))
	if err != nil {
		return fmt.Errorf("failed to find Cairo files: %w", err)
	}

	outputDir := filepath.Join(cfg.TempOutputDir, "compiler")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		fileNameNoExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		outputFile := filepath.Join(outputDir, fmt.Sprintf("juno_%s.contract_class.json", fileNameNoExt))

		cmd := exec.Command("starknet-compile", "--single-file", file, outputFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to compile %s: %w", file, err)
		}
	}

	return nil
}

func compileWithScarb(cfg Config) error {
	outputDir := filepath.Join(cfg.TempOutputDir, "scarb")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	scarbProjectDir := filepath.Dir(cfg.ScarbSrcDir)

	cmd := exec.Command("scarb", "--target-dir", cfg.TempOutputDir, "build")
	cmd.Dir = scarbProjectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to compile Scarb project: %w", err)
	}

	return nil
}

func compareCompilerArtifacts(cfg Config) error {
	storedFiles, err := filepath.Glob(filepath.Join(cfg.CompilerTargetDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to find stored artifacts: %w", err)
	}

	generatedDir := filepath.Join(cfg.TempOutputDir, "compiler")

	for _, storedFile := range storedFiles {
		fileName := filepath.Base(storedFile)
		generatedFile := filepath.Join(generatedDir, fileName)

		if err := compareJsonFiles(storedFile, generatedFile); err != nil {
			return fmt.Errorf("comparison failed for %s: %w", fileName, err)
		}
	}

	return nil
}

func compareScarbArtifacts(cfg Config) error {
	storedFiles, err := filepath.Glob(filepath.Join(cfg.ScarbTargetDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to find stored artifacts: %w", err)
	}

	generatedDir := filepath.Join(cfg.TempOutputDir, "dev")

	for _, storedFile := range storedFiles {
		fileName := filepath.Base(storedFile)
		generatedFile := filepath.Join(generatedDir, fileName)

		if err := compareJsonFiles(storedFile, generatedFile); err != nil {
			return fmt.Errorf("comparison failed for %s: %w", fileName, err)
		}
	}

	return nil
}

func compareJsonFiles(file1, file2 string) error {
	data1, err := os.ReadFile(file1)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file1, err)
	}

	data2, err := os.ReadFile(file2)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file2, err)
	}

	var json1, json2 interface{}
	if err := json.Unmarshal(data1, &json1); err != nil {
		return fmt.Errorf("failed to parse JSON from %s: %w", file1, err)
	}
	if err := json.Unmarshal(data2, &json2); err != nil {
		return fmt.Errorf("failed to parse JSON from %s: %w", file2, err)
	}

	// Compare the normalized JSON
	if !jsonEqual(json1, json2) {
		return fmt.Errorf("artifacts do not match. Expected: %s, Got: %s", file1, file2)
	}

	return nil
}

func jsonEqual(a, b interface{}) bool {
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return string(aBytes) == string(bBytes)
}

func TestCompilerArtifacts(t *testing.T) {
	cfg := loadConfig()

	if err := os.MkdirAll(filepath.Join(cfg.TempOutputDir, "compiler"), 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	if err := compileWithCairoCompiler(cfg); err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	if err := compareCompilerArtifacts(cfg); err != nil {
		t.Fatalf("Verification failed: %v", err)
	}
}

func TestScarbArtifacts(t *testing.T) {
	cfg := loadConfig()

	if err := os.MkdirAll(cfg.TempOutputDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	if err := compileWithScarb(cfg); err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	if err := compareScarbArtifacts(cfg); err != nil {
		t.Fatalf("Verification failed: %v", err)
	}
}
