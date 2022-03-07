# 实验3. ：测试chunking size的影响

## 分析

1. 比较不同的chunking size(256KB, 128KB, 64KB, 16KB)下，我们的add的时间和get的时间的区别。
2. 只有ipfs
3. 一个provider测试上传的情况；一个client，测试remote get的情况
4. 所以我们就比较四组数据即可？

## ipfs

### on provider:

```sh
cd local-node
./main -c upload -n 100 -s 16777216 -cid cids -chunker size-262144
./main -c daemon
```

### on client

```sh
./main -c downloads -cid cids
```

然后修改`size-262144`为`size-131072`, `size-65536`, `size-32768`, `size-16384`