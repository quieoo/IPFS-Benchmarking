import { create } from 'kubo-rpc-client';
import crypto from 'crypto';
import { performance } from 'perf_hooks';
import fs from 'fs/promises'; // 使用 promise 版本的 fs 模块

// 生成随机内容的函数
const generateRandomData = (size) => {
  return crypto.randomBytes(size); // 生成指定大小的随机字节数据
};

(async (fileSizeInBytes, numberOfFiles) => {
  // 连接到 IPFS API
  const client = create(new URL('http://127.0.0.1:5001'));

  try {
    let totalUploadTime = 0;
    let cids = [];

    for (let i = 1; i <= numberOfFiles; i++) {
      // 生成随机内容
      const randomData = generateRandomData(fileSizeInBytes);

      // 开始记录上传时间
      const startTime = performance.now();

      // 上传随机内容到IPFS
      const { cid } = await client.add(randomData);

      // 结束时间
      const endTime = performance.now();
      const uploadTime = endTime - startTime;

      console.log(`Random content ${i} added with CID: ${cid.toString()} in ${uploadTime.toFixed(2)} ms`);

      // 累加总上传时间
      totalUploadTime += uploadTime;

      // 将 CID 保存到数组中
      cids.push(cid.toString());
    }

    // 将所有 CID 写入文件
    await fs.writeFile('cids.txt', cids.join('\n'), 'utf-8');
    console.log('CIDs saved to cids.txt');

    // 计算并输出平均上传时间
    const averageUploadTime = totalUploadTime / numberOfFiles;
    console.log(`Average upload time: ${averageUploadTime.toFixed(2)} ms`);

  } catch (error) {
    console.error('Error adding content to IPFS:', error);
  }
})(4 * 1024, 3);  // 参数：文件大小4KB，文件数量3个
