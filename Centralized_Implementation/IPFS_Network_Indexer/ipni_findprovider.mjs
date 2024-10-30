import fetch from 'node-fetch';
import { performance } from 'perf_hooks';
import fs from 'fs/promises';

const qps = parseInt(process.argv[2], 10) || 1; // QPS参数，默认为1
const cidsFile = process.argv[3] || 'cids.txt'; // 从命令行获取文件路径，默认为 'cids.txt'
const printInterval = 1000; // 每隔5秒打印统计信息

// 从文件读取 CID 列表并打乱顺序
async function readCidsFromFile(filename) {
  try {
    const data = await fs.readFile(filename, 'utf-8');
    const lines = data.split('\n').filter(Boolean); // 过滤空行
    return shuffleArray(lines); // 打乱行的顺序并返回
  } catch (error) {
    console.error(`Error reading file ${filename}:`, error);
    throw error;
  }
}

// 打乱数组顺序的函数
function shuffleArray(array) {
  for (let i = array.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [array[i], array[j]] = [array[j], array[i]]; // 交换元素
  }
  return array;
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
    const data= await response.text();
    // console.log(data);

    const endTime = performance.now(); // End time
    const executionTime = endTime - startTime; // Calculate execution time

    return executionTime; // Return execution time

  } catch (error) {
    console.error(`Error finding providers for CID ${cidString}:`, error);
    return 0; // If failed, return 0 as execution time
  }
}

(async () => {
  try {
    // Read CIDs from 'cids.txt'
    const cids = await readCidsFromFile(cidsFile);
    console.log(`Found ${cids.length} CIDs to search for providers.`);

    let totalExecutionTime = 0;
    let completedRequests = 0;
    let requestsSent = 0;
    let currentIndex = 0;
    const executionTimes = [];
    let activeRequests = 0; // 追踪当前进行中的请求数量

    // Function to stop both timers when all CIDs are processed AND all requests are completed
    const stopTimersIfNeeded = () => {
      if (currentIndex >= cids.length && activeRequests === 0) { // 保证所有请求都完成
        clearInterval(requestIntervalId); // 停止发送请求的定时器
        clearInterval(printIntervalId); // 停止打印统计的定时器
        console.log('All requests completed.');
      }
    };

    // 定时器1：每秒发送 qps 个请求
    const requestIntervalId = setInterval(async () => {
      if (currentIndex >= cids.length) {
        // 如果所有请求已发出，不再发送请求，直接检查是否可以停止定时器
        stopTimersIfNeeded();
        return;
      }

      const promises = [];

      for (let i = 0; i < qps; i++) {
        if (currentIndex >= cids.length) {
          break; // 停止发出新的请求
        }

        const cid = cids[currentIndex];
        currentIndex++; // 更新 CID 的索引
        requestsSent++;
        activeRequests++; // 增加活动请求计数

        // 把每个请求的 Promise 存入数组，稍后执行 Promise.all
        promises.push(findProvidersForCID(cid).then(executionTime => {
          if (executionTime > 0) {
            totalExecutionTime += executionTime;
            executionTimes.push(executionTime); // 存储每个请求的执行时间
            completedRequests++;
          }
          activeRequests--; // 每个请求完成时减少活动请求计数
          stopTimersIfNeeded(); // 检查是否需要停止定时器
        }));
      }

      // 等待所有请求完成
      await Promise.all(promises);

    }, 1000); // 每秒发送 qps 个请求

    // 定时器2：定期打印统计信息
    const printIntervalId = setInterval(() => {
      if (completedRequests > 0) {
        const averageExecutionTime = totalExecutionTime / completedRequests;
        const throughput = completedRequests / (requestsSent / qps);

        console.log(`Requests sent: ${requestsSent}`);
        console.log(`Completed requests: ${completedRequests}`);
        console.log(`Average execution time: ${averageExecutionTime.toFixed(2)} ms`);
        console.log(`Throughput: ${throughput.toFixed(2)} requests/second`);
      }

      // 检查定时器是否需要停止
      stopTimersIfNeeded();

    }, printInterval); // 每隔 5 秒打印一次统计信息

  } catch (error) {
    console.error('Error:', error);
  }
})();
