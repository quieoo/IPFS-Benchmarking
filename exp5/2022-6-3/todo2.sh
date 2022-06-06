rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt;

./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.97 -rmn nb -loadsavecache > acn-0_97.out ;

rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt;

./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0 -rmn nb -loadsavecache > acn-0.out ;

rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt;

./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.01 -rmn nb -loadsavecache > acn-0_01.out ;

rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt;

./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.5 -rmn nb -loadsavecache > acn-0_5.out ;

rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt;

./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.7 -rmn nb -loadsavecache > acn-0_7.out ;

rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt;

./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.9 -rmn nb -loadsavecache > acn-0_9.out ;

rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt;

./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.95 -rmn nb -loadsavecache > acn-0_95.out ;

rm ../.ipfs -rf; ./../ipfs init; cp cache.txt.converge cache.txt;

./localIPFSNode -c downloads -cid acid-new -enablemetrics -PeerRH -B 0.99 -rmn nb -loadsavecache > acn-0_99.out ;

