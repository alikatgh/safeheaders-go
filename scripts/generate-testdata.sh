#!/bin/bash
# Generate test data files for SafeHeaders-Go

set -e

echo "Generating test data for SafeHeaders-Go..."
echo ""

TESTDATA_DIR="testdata"
mkdir -p "$TESTDATA_DIR"

# Generate large JSON file (10MB)
echo "Generating large.json (10MB)..."
{
    echo "["
    for i in {1..50000}; do
        if [ $i -gt 1 ]; then echo ","; fi
        cat << EOF
  {
    "id": $i,
    "name": "Object $i",
    "description": "This is a test object with id $i for benchmarking purposes",
    "value": $(($i * 10)),
    "active": true,
    "tags": ["tag1", "tag2", "tag3"],
    "metadata": {
      "created": "2025-01-01T00:00:00Z",
      "modified": "2025-01-01T00:00:00Z",
      "version": "1.0"
    }
  }
EOF
    done
    echo ""
    echo "]"
} > "$TESTDATA_DIR/large.json"

echo "✓ Generated large.json ($(du -h $TESTDATA_DIR/large.json | cut -f1))"

# Generate large XML file (5MB)
echo "Generating large.xml (5MB)..."
{
    echo '<?xml version="1.0" encoding="UTF-8"?>'
    echo '<root>'
    for i in {1..10000}; do
        cat << EOF
  <item id="$i" name="Item $i">
    <description>This is a test item with id $i for benchmarking purposes</description>
    <value>$(($i * 10))</value>
    <active>true</active>
    <tags>
      <tag>tag1</tag>
      <tag>tag2</tag>
      <tag>tag3</tag>
    </tags>
    <metadata created="2025-01-01T00:00:00Z" modified="2025-01-01T00:00:00Z" version="1.0"/>
  </item>
EOF
    done
    echo '</root>'
} > "$TESTDATA_DIR/large.xml"

echo "✓ Generated large.xml ($(du -h $TESTDATA_DIR/large.xml | cut -f1))"

# Generate nested JSON for testing deep recursion
echo "Generating nested.json..."
{
    echo -n '{"level": 0'
    for i in {1..100}; do
        echo -n ', "nested": {"level": '$i
    done
    for i in {1..100}; do
        echo -n '}'
    done
    echo '}'
} > "$TESTDATA_DIR/nested.json"

echo "✓ Generated nested.json ($(du -h $TESTDATA_DIR/nested.json | cut -f1))"

# Generate array of primitives
echo "Generating primitives.json..."
{
    echo -n '['
    for i in {1..10000}; do
        if [ $i -gt 1 ]; then echo -n ', '; fi
        case $((i % 4)) in
            0) echo -n "$i" ;;
            1) echo -n '"string'$i'"' ;;
            2) echo -n 'true' ;;
            3) echo -n 'null' ;;
        esac
    done
    echo ']'
} > "$TESTDATA_DIR/primitives.json"

echo "✓ Generated primitives.json ($(du -h $TESTDATA_DIR/primitives.json | cut -f1))"

echo ""
echo "Test data generation complete!"
echo "Files created in $TESTDATA_DIR/:"
ls -lh "$TESTDATA_DIR" | tail -n +2
