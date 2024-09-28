import { create } from 'ipfs-http-client';
import { CID } from 'multiformats/cid';
import fs from 'fs/promises';
import { performance } from 'perf_hooks';
import fetch from 'node-fetch';  // You might need to install this if not available

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

// 调用 /api/v0/routing/findprovs 获取 providers，并记录执行时间
async function findProvidersForCID(cidString, timeout) {
  try {
    const startTime = performance.now(); // 开始时间

    // API URL
    const url = `http://127.0.0.1:5001/api/v0/routing/findprovs?arg=${cidString}&timeout=${timeout}ms`;

    // 发送请求
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Error fetching providers: ${response.statusText}`);
    }

    const data = await response.json();

    // 处理结果
    let providerCount = 0;
    for (const provider of data) {
      console.log(`Provider for CID ${cidString} found: ${provider.ID}`);
      providerCount++;
    }

    const endTime = performance.now(); // 结束时间
    const executionTime = endTime - startTime; // 计算执行时间

    console.log(`FindProvs for CID ${cidString} completed in ${executionTime.toFixed(2)} ms`);
    return executionTime; // 返回执行时间

  } catch (error) {
    console.error(`Error finding providers for CID ${cidString}:`, error);
    return 0; // 如果失败，返回0作为执行时间
  }
}

(async () => {
  try {
    // 读取 'cids.txt' 中的 CID 列表
    const cids = await readCidsFromFile('cids.txt');
    console.log(`Found ${cids.length} CIDs to search for providers.`);

    let totalExecutionTime = 0;
    
    // 设置超时时间，单位为毫秒
    const timeout = 10000; // 10秒

    // 逐个 CID 获取 providers，并累加执行时间
    for (let i = 0; i < cids.length; i++) {
      const cid = cids[i];
      console.log(`Searching for providers of CID: ${cid}`);
      const executionTime = await findProvidersForCID(cid, timeout);
      totalExecutionTime += executionTime;
    }

    // 计算并输出平均执行时间
    const averageExecutionTime = totalExecutionTime / cids.length;
    console.log(`Average findProvs execution time: ${averageExecutionTime.toFixed(2)} ms`);

  } catch (error) {
    console.error('Error:', error);
  }
})();
