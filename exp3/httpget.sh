serverIP="101.42.137.157"
clientIP=(
	"123.57.175.180"
	#"127.0.0.1"
)


# 并行下载文件
for((i=0;i<${#clientIP[@]};i++));
do
	ssh benchmark@${clientIP[$i]} "cd httpfs; ./client -c download -fn filenames -h ${serverIP} -cg 8" &
done

wait

