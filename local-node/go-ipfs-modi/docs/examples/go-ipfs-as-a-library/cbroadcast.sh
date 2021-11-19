#!/bin/bash



targets=('embed78' 'embed67' 'embed66' 'embed79' 'embed61' 'embed62' 'embed64' 'embed65')
addrs=('47.104.30.111' '47.241.161.123' '47.252.27.144' '147.139.35.43' '147.139.165.174' '47.74.34.159' '8.208.100.173' '47.91.106.95')

length=${#addrs[@]}

for ((k=0;k<$length;k++))
do
 expect expectmain.sh 'root' ${addrs[$k]} '2'
done

