import { create } from 'kubo-rpc-client';
import crypto from 'crypto';
import { performance } from 'perf_hooks';
import fs from 'fs/promises';
import { fileURLToPath } from 'url';
import { dirname } from 'path';

// 生成随机内容的函数
const generateRandomData = (size) => {
  return crypto.randomBytes(size);
};

// 获取当前文件的路径和目录
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// 保存上传状态
let uploadedFiles = 0;
let totalUploadTime = 0;
let cids = [];
let uploadInterval; // 定义在全局，以便在 SIGINT 时能停止上传

// 上传文件的函数
const uploadFile = async (fileSizeInBytes, requestCount, rps) => {
  // 连接到 IPFS API
  const client = create(new URL('http://127.0.0.1:5001'));

  // 每秒上传请求
  const interval = 1000 / rps;
  let isUploading = false; // 用于跟踪上传状态的标志位

  return new Promise((resolve, reject) => {
    const uploadInterval = setInterval(async () => {
      try {
        // 确保上一个请求已经完成
        if (isUploading) {
          return;
        }
        isUploading = true;

        // 检查是否达到了上传的请求数
        if (requestCount !== 0 && uploadedFiles >= requestCount) {
            clearInterval(uploadInterval); // 停止上传
            const averageUploadTime = totalUploadTime / uploadedFiles;
            console.log(`All files uploaded. Average upload time: ${averageUploadTime.toFixed(2)} ms`);
            await fs.writeFile('cids.txt', cids.join('\n'), 'utf-8');
            console.log('CIDs saved to cids.txt');
            resolve();
            return;
        }

        const randomData = generateRandomData(fileSizeInBytes);
        const startTime = performance.now();
        const { cid } = await client.add(randomData);
        const endTime = performance.now();
        const uploadTime = endTime - startTime;

        uploadedFiles++;
        totalUploadTime += uploadTime;

        // 每上传100个文件后输出日志
        if (uploadedFiles % 100 === 0) {
            const currentAverageUploadTime = totalUploadTime / uploadedFiles;
            console.log(`Uploaded ${uploadedFiles} files. Current average upload time: ${currentAverageUploadTime.toFixed(2)} ms`);
        }

        // 将 CID 添加到列表
        cids.push(cid.toString());

      } catch (error) {
        console.error(`Error during file upload: ${error.message}`);
        clearInterval(uploadInterval);
        reject(error);
      } finally {
        isUploading = false; // 无论是否出错，解锁
      }
    }, interval);
  });
};


// 捕获 SIGINT（例如 Ctrl+C）信号以优雅地终止程序，并输出平均延迟
process.on('SIGINT', async () => {
  console.log('\nStopping upload...');

  if (uploadInterval) {
    clearInterval(uploadInterval); // 停止上传
  }

  if (uploadedFiles > 0) {
    const averageUploadTime = totalUploadTime / uploadedFiles;
    console.log(`Average upload time before termination: ${averageUploadTime.toFixed(2)} ms`);
  } else {
    console.log('No files were uploaded.');
  }

  // 将已上传的 CID 写入文件
  if (cids.length > 0) {
    await fs.writeFile('cids.txt', cids.join('\n'), 'utf-8');
    console.log('CIDs saved to cids.txt');
  }

  process.exit();
});

// 主逻辑：根据 RPS 和文件大小进行上传
(async () => {
  // 获取命令行参数
  const args = process.argv.slice(2);
  const rps = parseInt(args[0], 10);   // 默认1个请求/秒
  const fileSizeInBytes = parseInt(args[1], 10); // 默认4KB文件大小
  const requestCount = parseInt(args[2], 10); // 默认上传10个文件，0表示无限

  console.log(`Starting upload with RPS: ${rps}, ${fileSizeInBytes} bytes per file, ${requestCount === 0 ? 'unlimited' : requestCount} files`);

  // 启动上传
  try {
    await uploadFile(fileSizeInBytes, requestCount, rps);
  } catch (error) {
    console.error('Upload process failed:', error);
  }
})();
