# 测试IPFS的 PeerResponseHistory 2022年5月31日

neighbour ID `12D3KooWJzyALnC9trKzCNQxcZt8VS4zUtv3fQvr8zfPKfcnP62P`

## upload

1. `nohup ./localIPFSNode -c upload -n 500 -s 4m -provideEvery -cid cid-new &`
2. 将 cid 文件分成多份，可以在`2022-6-3/`下找到最终使用的cid文件
3. 使用的`localIPFSNode`为 `localIPFSNode_upload`；这个是一个旧的版本，当初还没有`provideeach`选项，我自己写了一个`provideEvery`，思路类似.


## download

1. *.out, cache.txt.converge cid 文件`2022-6-3`下
2. 使用的`localIPFSNode`为`localIPFSNode_download`

用到的命令

1. 预处理(每次download都运行一次)：
   1. `rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt`
2. b = 不查
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -rmn nb  > acn.out &`
3. b = 0
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0 -rmn nb -loadsavecache > acn-0.out &`
4. b = 0.01
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.01 -rmn nb -loadsavecache > acn-0_01.out &`
5. b = 0.5
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.5 -rmn nb -loadsavecache > acn-0_5.out &`
6. b = 0.7
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.7 -rmn nb -loadsavecache > acn-0_7.out &`
7. b = 0.9
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.9 -rmn nb -loadsavecache > acn-0_9.out &`
8. b = 0.95
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.95 -rmn nb -loadsavecache > acn-0_95.out &`
9. b = 0.97
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.97 -rmn nb -loadsavecache > acn-0_97.out &`
10. b = 0.99
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.99 -rmn nb -loadsavecache > acn-0_99.out &`

# 下面是旧的一些尝试

## 验证查&更新 cache 不会带来太多的开销

1. 查cache但不使用
   1. `nohup ./localIPFSNode -c downloads -cid cid0 -enablemetrics -PeerRH -B 0 -rmn nb > c0-0.out &`
2. 不查cache
   1. `nohup ./localIPFSNode -c downloads -cid cid0 -enablemetrics -rmn nb > c0.out &`

## 测试合适的参数

1. `0.01` 几乎不参考——影响只在公共前缀相同时产生
   1. `nohup ./localIPFSNode -c downloads -cid cid0 -enablemetrics -PeerRH -B 0.01 -rmn nb > c0-0_01.out &`
2. `0.1`参考比较多
   1. `nohup ./localIPFSNode -c downloads -cid cid0 -enablemetrics -PeerRH -B 0.1 -rmn nb > c0-0_1.out &`
3. `0.5`一半一半
   1. `nohup ./localIPFSNode -c downloads -cid cid0 -enablemetrics -PeerRH -B 0.5 -rmn nb > c0-0_5.out &`

## 在某个合适的参数上，不断提高组数，得到更高的命中率

假设是`0.3`

1. `nohup ./localIPFSNode -c downloads -cid cid0 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec0-0_3.out &`
2. `nohup ./localIPFSNode -c downloads -cid cid1 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec1-0_3.out &`
3. `nohup ./localIPFSNode -c downloads -cid cid2 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec2-0_3.out &`
4. `nohup ./localIPFSNode -c downloads -cid cid3 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec3-0_3.out &`
5. `nohup ./localIPFSNode -c downloads -cid cid4 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec4-0_3.out &`
6. `nohup ./localIPFSNode -c downloads -cid cid5 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec5-0_3.out &`
7. `nohup ./localIPFSNode -c downloads -cid cid6 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec6-0_3.out &`
8. `nohup ./localIPFSNode -c downloads -cid cid7 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec7-0_3.out &`
9. `nohup ./localIPFSNode -c downloads -cid cid8 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec8-0_3.out &`
10. `nohup ./localIPFSNode -c downloads -cid cid9 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > ec9-0_3.out &`

## 我们发现得到了高命中率，但是效果不显著

思考调整的方法：

1. 调整的 b 的取值查看是否会改善 ？
2. 
3. 试图将 dial 的时间也参考进来 ——同时使用 dial 和 query？
4. 或者说只用 dial 的时间 ？

尝试 b  的取值

1. b = 0.01
   1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.01 -rmn nb -loadsavecache > cn-0_01.out &`

2. b = 0.5
   1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.5 -rmn nb -loadsavecache > cn-0_5.out &`
3. b = 0.7
   1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.7 -rmn nb -loadsavecache > cn-0_7.out &`
4. b = 0.8
   1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.8 -rmn nb -loadsavecache > cn-0_8.out &`
5. b = 0.9
   1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.9 -rmn nb -loadsavecache > cn-0_9.out &`

6. b = 0.75
   1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.75 -rmn nb -loadsavecache > cn-0_75.out &`
7. b = 0.001
   1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.001 -rmn nb -loadsavecache > cn-0_001.out &`
8. b = 0.95
   1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.95 -rmn nb -loadsavecache > cn-0_95.out &`
9. b = 0.99
   1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.99 -rmn nb -loadsavecache > cn-0_99.out &`
10. b = 0.97
    1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0.97 -rmn nb -loadsavecache > cn-0_97.out &`
11. b = 0
    1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -PeerRH -B 0 -rmn nb -loadsavecache > cn-0.out &`
12. 不查
    1. `nohup ./localIPFSNode -c downloads -cid cid-new -enablemetrics -rmn nb  > cn.out &`

## 尝试让命中率收敛

1. 先去upload足够多的文件再搞300个吧！
   1. `nohup ./localIPFSNode -c upload -n 300 -s 4m -provideEvery -cid cid-add &`
2. 然后再去 get 
   1. `nohup ./localIPFSNode -c downloads -cid acid0 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > eac0-0_3.out &`
   2. `nohup ./localIPFSNode -c downloads -cid acid1 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > eac1-0_3.out &`
   3. `nohup ./localIPFSNode -c downloads -cid acid2 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > eac2-0_3.out &`
   4. `nohup ./localIPFSNode -c downloads -cid acid3 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > eac3-0_3.out &`
   5. `nohup ./localIPFSNode -c downloads -cid acid4 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > eac4-0_3.out &`
   6. `nohup ./localIPFSNode -c downloads -cid acid5 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > eac5-0_3.out &`
   7. `nohup ./localIPFSNode -c downloads -cid acid6 -enablemetrics -PeerRH -B 0.3 -rmn nb -loadsavecache > eac6-0_3.out &`

## 再一次寻找好的b的取值（只不过这次用收敛的cache）

1. 预处理：
   1. `rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt`
2. b = 不查
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -rmn nb  > acn.out &`
3. b = 0
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0 -rmn nb -loadsavecache > acn-0.out &`
4. b = 0.01
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.01 -rmn nb -loadsavecache > acn-0_01.out &`
5. b = 0.5
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.5 -rmn nb -loadsavecache > acn-0_5.out &`
6. b = 0.7
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.7 -rmn nb -loadsavecache > acn-0_7.out &`
7. b = 0.9
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.9 -rmn nb -loadsavecache > acn-0_9.out &`
8. b = 0.95
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.95 -rmn nb -loadsavecache > acn-0_95.out &`
9. b = 0.97
   1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.97 -rmn nb -loadsavecache > acn-0_97.out &`
10. b = 0.99
    1. `nohup ./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.99 -rmn nb -loadsavecache > acn-0_99.out &`





