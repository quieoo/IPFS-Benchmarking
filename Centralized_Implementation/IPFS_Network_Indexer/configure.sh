#!/bin/bash

# niRouting and dhtRouting definitions
# change the "Endpoint" filed to your indexer's address
niRouting='{
    "Methods": {
        "find-peers": { "RouterName": "WanDHT" },
        "find-providers": { "RouterName": "ParallelHelper" },
        "get-ipns": { "RouterName": "WanDHT" },
        "provide": { "RouterName": "ParallelHelper" },
        "put-ipns": { "RouterName": "WanDHT" }
    },
    "Routers": {
        "IndexProvider": {
        "Parameters": {
            "Endpoint": "http://47.237.17.55:50617", 
            "MaxProvideBatchSize": 10000,
            "MaxProvideConcurrency": 1
        },
        "Type": "http"
        },
        "ParallelHelper": {
        "Parameters": {
            "Routers": [
            {
                "IgnoreErrors": true,
                "RouterName": "IndexProvider",
                "Timeout": "30m"
            }
            ]
        },
        "Type": "parallel"
        },
        "WanDHT": {
        "Parameters": {
            "AcceleratedDHTClient": false,
            "Mode": "auto",
            "PublicIPNetwork": true
        },
        "Type": "dht"
        }
    },
    "Type": "custom"
}'

dhtRouting='{
    "Routers": null,
    "Methods": null
}'

# Function to modify file
modify_file() {
  local filename="$1"
  local mod_type="$2"

  if [[ ! -f "$filename" ]]; then
    echo "File not found!"
    exit 1
  fi

  # Read the file content
  jsonData=$(cat "$filename")

  # Choose routing modification based on mod_type
  if [[ "$mod_type" == "ni" ]]; then
    routing="$niRouting"
  elif [[ "$mod_type" == "dht" ]]; then
    routing="$dhtRouting"
  else
    echo "Invalid modification type. Use 'ni' or 'dht'."
    exit 1
  fi

  # Modify the JSON file using jq
  newData=$(echo "$jsonData" | jq --argjson routing "$routing" '.Routing = $routing')

  # Write the modified data back to the file
  echo "$newData" > "$filename"

  echo "File updated successfully!"
}

# Check command line arguments
if [[ "$#" -ne 2 ]]; then
  echo "Usage: $0 <filename> <modification-type>"
  exit 1
fi

modify_file "$1" "$2"
