#!/bin/bash

BASE_DIR="test_upload_tree"
mkdir -p "$BASE_DIR"

echo "Generating random tree in $BASE_DIR..."

# Max depth of 2 folders
mkdir -p "$BASE_DIR/folder1/subfolder1"
mkdir -p "$BASE_DIR/folder2"

# Max 5-6 files total
NUM_FILES=$((5 + RANDOM % 2))

# Array of available directories to randomly place files
DIRS=("$BASE_DIR" "$BASE_DIR/folder1" "$BASE_DIR/folder1/subfolder1" "$BASE_DIR/folder2")

for i in $(seq 1 $NUM_FILES); do
    # Pick a random directory
    DIR=${DIRS[$RANDOM % ${#DIRS[@]}]}
    
    # Random size between 10MB and 25MB (in 1MB blocks)
    SIZE_MB=$((10 + RANDOM % 16))
    FILE_NAME="random_file_$i.bin"
    
    echo "Creating $DIR/$FILE_NAME of size ${SIZE_MB}MB..."
    
    # Generate random gibberish file using /dev/urandom
    dd if=/dev/urandom of="$DIR/$FILE_NAME" bs=1M count=$SIZE_MB status=none
done

echo "Done! Generated $NUM_FILES files."
echo "---------------------------------"
# Print tree and file sizes
find "$BASE_DIR" -type f -exec ls -lh {} \;
