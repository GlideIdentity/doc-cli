#!/bin/bash
set -e

CLI="./gdrive-bench"
DOCS_DIR="./docs"
INDEX_FILE="$DOCS_DIR/index.json"

if [ -z "$GDRIVE_FOLDER" ]; then
  echo "Error: GDRIVE_FOLDER not set"
  exit 1
fi

echo "Uploading documents to Drive folder: $GDRIVE_FOLDER"
echo ""

# Read each doc from index.json and upload
TEMP_INDEX=$(mktemp)
cp "$INDEX_FILE" "$TEMP_INDEX"

for file in "$DOCS_DIR"/*.md; do
  filename=$(basename "$file")
  title=$(head -1 "$file" | sed 's/^# //')
  content=$(cat "$file")

  echo "Uploading: $filename ($title)..."
  result=$($CLI create --name "$title" --body "$content" --folder "$GDRIVE_FOLDER" 2>&1)
  drive_id=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
  elapsed=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin)['_elapsed_ms'])")
  echo "  -> ID: $drive_id (${elapsed}ms)"

  # Update index.json with the drive ID
  python3 -c "
import json
with open('$INDEX_FILE') as f:
    idx = json.load(f)
for entry in idx:
    if entry['localPath'] == '$filename':
        entry['driveId'] = '$drive_id'
with open('$INDEX_FILE', 'w') as f:
    json.dump(idx, f, indent=2)
"
done

echo ""
echo "All documents uploaded. Updated index.json with Drive IDs."
cat "$INDEX_FILE"
