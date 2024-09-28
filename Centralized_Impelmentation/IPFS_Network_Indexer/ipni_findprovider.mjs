import fetch from 'node-fetch';
import { performance } from 'perf_hooks';
import fs from 'fs/promises';

// Read 'cids.txt' for CID list
async function readCidsFromFile(filename) {
  try {
    const data = await fs.readFile(filename, 'utf-8');
    return data.split('\n').filter(Boolean); // Filter out empty lines
  } catch (error) {
    console.error(`Error reading file ${filename}:`, error);
    throw error;
  }
}

// Send GET request to /api/v0/routing/findprovs to fetch providers
async function findProvidersForCID(cidString, timeout) {
  try {
    const startTime = performance.now(); // Start time

    // Construct the GET URL
    const url = `http://127.0.0.1:5001/api/v0/routing/findprovs?arg=${cidString}&timeout=${timeout}ms`;

    // Send GET request
    const response = await fetch(url, {
      method: 'GET'
    });

    if (!response.ok) {
      throw new Error(`Error fetching providers: ${response.statusText}`);
    }

    const data = await response.json();

    // Process response data
    let providerCount = 0;
    for (const provider of data) {
      console.log(`Provider for CID ${cidString} found: ${provider.ID}`);
      providerCount++;
    }

    const endTime = performance.now(); // End time
    const executionTime = endTime - startTime; // Calculate execution time

    console.log(`FindProvs for CID ${cidString} completed in ${executionTime.toFixed(2)} ms`);
    return executionTime; // Return execution time

  } catch (error) {
    console.error(`Error finding providers for CID ${cidString}:`, error);
    return 0; // If failed, return 0 as execution time
  }
}

(async () => {
  try {
    // Read CIDs from 'cids.txt'
    const cids = await readCidsFromFile('cids.txt');
    console.log(`Found ${cids.length} CIDs to search for providers.`);

    let totalExecutionTime = 0;
    
    // Set timeout in milliseconds
    const timeout = 10000; // 10 seconds

    // Fetch providers for each CID and accumulate execution time
    for (let i = 0; i < cids.length; i++) {
      const cid = cids[i];
      console.log(`Searching for providers of CID: ${cid}`);
      const executionTime = await findProvidersForCID(cid, timeout);
      totalExecutionTime += executionTime;
    }

    // Calculate and output average execution time
    const averageExecutionTime = totalExecutionTime / cids.length;
    console.log(`Average findProvs execution time: ${averageExecutionTime.toFixed(2)} ms`);

  } catch (error) {
    console.error('Error:', error);
  }
})();
