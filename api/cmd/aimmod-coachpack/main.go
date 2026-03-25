package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/veryCrunchy/aimmod-hub/api/internal/coaching"
)

func main() {
	outDir := flag.String("out-dir", "", "directory to write manifest.json and knowledge.v1.json into")
	flag.Parse()

	base, err := coaching.Load()
	if err != nil {
		exitf("load coaching content: %v", err)
	}
	manifest, err := coaching.GetManifest()
	if err != nil {
		exitf("build coaching manifest: %v", err)
	}

	if *outDir == "" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(base); err != nil {
			exitf("write coaching pack: %v", err)
		}
		return
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		exitf("create output directory: %v", err)
	}

	if err := writeJSON(filepath.Join(*outDir, "knowledge.v1.json"), base); err != nil {
		exitf("write knowledge pack: %v", err)
	}
	if err := writeJSON(filepath.Join(*outDir, "manifest.json"), manifest); err != nil {
		exitf("write manifest: %v", err)
	}

	fmt.Fprintf(os.Stdout, "wrote coaching pack to %s\n", *outDir)
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
