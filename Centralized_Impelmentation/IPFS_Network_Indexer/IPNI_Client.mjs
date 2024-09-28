import { create } from 'kubo-rpc-client';
import fs from 'fs/promises'; // 使用 promise 版本的 fs 模块
import { performance } from 'perf_hooks'; // 用于记录时间

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

// 下载文件并保存，记录下载时间
async function downloadFileFromIPFS(client, cid, fileIndex) {
  try {
    const chunks = [];
    const startTime = performance.now(); // 开始时间

    for await (const chunk of client.cat(cid)) {
      chunks.push(chunk);
    }

    const endTime = performance.now(); // 结束时间
    const downloadTime = endTime - startTime; // 计算下载耗时

    const fileContent = Buffer.concat(chunks);
    const filename = `downloaded_file_${fileIndex}.bin`;

    await fs.writeFile(filename, fileContent);
    console.log(`File ${cid} downloaded and saved as ${filename} in ${downloadTime.toFixed(2)} ms`);

    return downloadTime; // 返回下载时间

  } catch (error) {
    console.error(`Error downloading file for CID ${cid}:`, error);
    return 0; // 如果下载失败，返回0作为下载时间
  }
}

(async () => {
  // 连接到 IPFS API
  const client = create(new URL('http://127.0.0.1:5001'));

  try {
    // 读取 'cids.txt' 中的 CID 列表
    const cids = await readCidsFromFile('cids.txt');
    console.log(`Found ${cids.length} CIDs to download.`);

    let totalDownloadTime = 0;

    // 逐个下载 CID 对应的文件，并累加下载时间
    for (let i = 0; i < cids.length; i++) {
      const cid = cids[i];
      console.log(`Downloading file for CID: ${cid}`);
      const downloadTime = await downloadFileFromIPFS(client, cid, i + 1);
      totalDownloadTime += downloadTime;
    }

    // 计算并输出平均下载时间
    const averageDownloadTime = totalDownloadTime / cids.length;
    console.log(`Average download time: ${averageDownloadTime.toFixed(2)} ms`);

  } catch (error) {
    console.error('Error:', error);
  }
})();
