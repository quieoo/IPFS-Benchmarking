const fs = require('fs').promises;

const niRouting = {
  "Routing": {
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
          "Endpoint": "http://127.0.0.1:50617",
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
  }
};

const dhtRouting = {
  "Routing": {
    "Routers": null,
    "Methods": null
  }
};

async function modifyFile(filename, modType) {
  try {
    // Read the file
    const data = await fs.readFile(filename, 'utf-8');
    const jsonData = JSON.parse(data);

    // Modify based on type
    if (modType === 'ni') {
      jsonData["Routing"] = niRouting["Routing"];
    } else if (modType === 'dht') {
      jsonData["Routing"] = dhtRouting["Routing"];
    } else {
      console.error("Invalid modification type. Use 'ni' or 'dht'.");
      return;
    }

    // Write the modified data back to the file
    const newData = JSON.stringify(jsonData, null, 2);
    await fs.writeFile(filename, newData, 'utf-8');

    console.log("File updated successfully!");
  } catch (error) {
    console.error("Error:", error);
  }
}

// Check for valid command line arguments
const args = process.argv.slice(2);
if (args.length !== 2) {
  console.error("Usage: node modify.js <filename> <modification-type>");
} else {
  modifyFile(args[0], args[1]);
}
