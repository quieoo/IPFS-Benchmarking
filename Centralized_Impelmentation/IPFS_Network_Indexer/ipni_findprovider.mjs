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

// Send POST request to /api/v0/routing/findprovs to fetch providers
async function findProvidersForCID(cidString, numProviders = 20, verbose = false) {
    try {
      const startTime = performance.now(); // Start time
  
      // Construct the POST URL with the proper parameters
      const url = `http://127.0.0.1:5001/api/v0/routing/findprovs?arg=${cidString}&verbose=${verbose}&num-providers=${numProviders}`;
  
      // Send POST request
      const response = await fetch(url, {
        method: 'POST'
      });
  
      if (!response.ok) {
        throw new Error(`Error fetching providers: ${response.statusText}`);
      }
  
      // Attempt to get the response as text
      const data = await response.text();
      console.log(`Raw response: ${data}`);
  
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

    // Fetch providers for each CID and accumulate execution time
    for (let i = 0; i < cids.length; i++) {
      const cid = cids[i];
      console.log(`Searching for providers of CID: ${cid}`);
      const executionTime = await findProvidersForCID(cid);
      totalExecutionTime += executionTime;
    }

    // Calculate and output average execution time
    const averageExecutionTime = totalExecutionTime / cids.length;
    console.log(`Average findProvs execution time: ${averageExecutionTime.toFixed(2)} ms`);

  } catch (error) {
    console.error('Error:', error);
  }
})();
