import { create } from 'kubo-rpc-client';
import { CID } from 'multiformats/cid';
import fs from 'fs/promises';
import { performance } from 'perf_hooks';

// 读取 'cids.txt' 中的 CID 列表
async function readCidsFromFile(filename) {
  try {
    const data = await fs.readFile(filename, 'utf-8');
    return data.split('\n').filter(Boolean); // 过滤掉空行
  } catch (error) {
    console.error(`Error reading file ${filename}:`, error);
    throw error;
  }
}

// 调用 query 查找 CID 的 providers，记录执行时间
async function findProvidersForCID(client, cidString, index) {
  try {
    const cid = CID.parse(cidString);
    
    const startTime = performance.now(); // 开始时间

    const providers = client.dht.query(cid);
    let providerCount = 0;

    for await (const provider of providers) {
      console.log(`Provider for CID ${cidString} found: ${provider.id.toString()}`);
      providerCount++;
    }

    const endTime = performance.now(); // 结束时间
    const executionTime = endTime - startTime; // 计算执行时间

    console.log(`FindProvs for CID ${cidString} completed in ${executionTime.toFixed(2)} ms`);
    return { executionTime, providerCount };

  } catch (error) {
    console.error(`Error finding providers for CID ${cidString}:`, error);
    return { executionTime: 0, providerCount: 0 }; // 如果失败，返回0作为执行时间
  }
}

(async () => {
  // 连接到 IPFS API
  const client = create(new URL('http://127.0.0.1:5001'));

  try {
    // 读取 'cids.txt' 中的 CID 列表
    const cids = await readCidsFromFile('cids.txt');
    console.log(`Found ${cids.length} CIDs to search for providers.`);

    let totalExecutionTime = 0;
    let totalProvidersFound = 0;

    // 逐个 CID 获取 providers，并累加执行时间
    for (let i = 0; i < cids.length; i++) {
      const cid = cids[i];
      console.log(`Searching for providers of CID: ${cid}`);
      const { executionTime, providerCount } = await findProvidersForCID(client, cid, i + 1);
      totalExecutionTime += executionTime;
      totalProvidersFound += providerCount;
    }

    // 计算并输出平均执行时间
    const averageExecutionTime = totalExecutionTime / cids.length;
    console.log(`Average findProvs execution time: ${averageExecutionTime.toFixed(2)} ms`);
    console.log(`Total providers found: ${totalProvidersFound}`);

  } catch (error) {
    console.error('Error:', error);
  }
})();