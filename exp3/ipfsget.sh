clientIP=(
	"101.42.137.157"
	"123.57.175.180"
)


# 并行下载文件
for((i=0;i<${#clientIP[@]};i++));
do
	ssh benchmark@${clientIP[$i]} "cd local-node; ./localIPFSNode -c downloads -cid cids -cg 4" &
done

wait

