package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	testdataDir := "testdata"
	if err := os.MkdirAll(testdataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating testdata directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generating test data for SafeHeaders-Go...")
	fmt.Println()

	// Generate large JSON file (10MB)
	if err := generateLargeJSON(filepath.Join(testdataDir, "large.json")); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating large.json: %v\n", err)
		os.Exit(1)
	}

	// Generate large XML file (5MB)
	if err := generateLargeXML(filepath.Join(testdataDir, "large.xml")); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating large.xml: %v\n", err)
		os.Exit(1)
	}

	// Generate nested JSON
	if err := generateNestedJSON(filepath.Join(testdataDir, "nested.json")); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating nested.json: %v\n", err)
		os.Exit(1)
	}

	// Generate primitives JSON
	if err := generatePrimitivesJSON(filepath.Join(testdataDir, "primitives.json")); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating primitives.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Test data generation complete!")
	fmt.Println("Files created in", testdataDir+":")

	// List generated files
	entries, _ := os.ReadDir(testdataDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			info, _ := entry.Info()
			fmt.Printf("  %s (%d bytes)\n", entry.Name(), info.Size())
		}
	}
}

func generateLargeJSON(filename string) error {
	fmt.Println("Generating large.json (10MB)...")

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprint(f, "[\n")
	for i := 1; i <= 50000; i++ {
		if i > 1 {
			fmt.Fprint(f, ",\n")
		}
		fmt.Fprintf(f, `  {
    "id": %d,
    "name": "Object %d",
    "description": "This is a test object with id %d for benchmarking purposes",
    "value": %d,
    "active": true,
    "tags": ["tag1", "tag2", "tag3"],
    "metadata": {
      "created": "2025-01-01T00:00:00Z",
      "modified": "2025-01-01T00:00:00Z",
      "version": "1.0"
    }
  }`, i, i, i, i*10)
	}
	fmt.Fprint(f, "\n]\n")

	info, _ := os.Stat(filename)
	fmt.Printf("✓ Generated large.json (%d MB)\n", info.Size()/(1024*1024))
	return nil
}

func generateLargeXML(filename string) error {
	fmt.Println("Generating large.xml (5MB)...")

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(f, `<root>`)

	for i := 1; i <= 10000; i++ {
		fmt.Fprintf(f, `  <item id="%d" name="Item %d">
    <description>This is a test item with id %d for benchmarking purposes</description>
    <value>%d</value>
    <active>true</active>
    <tags>
      <tag>tag1</tag>
      <tag>tag2</tag>
      <tag>tag3</tag>
    </tags>
    <metadata created="2025-01-01T00:00:00Z" modified="2025-01-01T00:00:00Z" version="1.0"/>
  </item>
`, i, i, i, i*10)
	}

	fmt.Fprintln(f, `</root>`)

	info, _ := os.Stat(filename)
	fmt.Printf("✓ Generated large.xml (%d MB)\n", info.Size()/(1024*1024))
	return nil
}

func generateNestedJSON(filename string) error {
	fmt.Println("Generating nested.json...")

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprint(f, `{"level": 0`)
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(f, `, "nested": {"level": %d`, i)
	}
	for i := 1; i <= 100; i++ {
		fmt.Fprint(f, `}`)
	}
	fmt.Fprint(f, "}\n")

	info, _ := os.Stat(filename)
	fmt.Printf("✓ Generated nested.json (%d KB)\n", info.Size()/1024)
	return nil
}

func generatePrimitivesJSON(filename string) error {
	fmt.Println("Generating primitives.json...")

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprint(f, "[")
	for i := 1; i <= 10000; i++ {
		if i > 1 {
			fmt.Fprint(f, ", ")
		}
		switch i % 4 {
		case 0:
			fmt.Fprintf(f, "%d", i)
		case 1:
			fmt.Fprintf(f, `"string%d"`, i)
		case 2:
			fmt.Fprint(f, "true")
		case 3:
			fmt.Fprint(f, "null")
		}
	}
	fmt.Fprint(f, "]\n")

	info, _ := os.Stat(filename)
	fmt.Printf("✓ Generated primitives.json (%d KB)\n", info.Size()/1024)
	return nil
}
