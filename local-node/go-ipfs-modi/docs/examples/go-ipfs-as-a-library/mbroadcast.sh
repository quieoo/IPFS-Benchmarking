#!/bin/bash



targets=('embed62' 'embed63' 'embed65' 'embed66' 'embed67' 'embed77' 'embed79' 'embed94')
addrs=('192.168.1.122' '192.168.1.100' '192.168.1.179' '192.168.1.131' '192.168.1.129' '192.168.1.101' '192.168.1.121' '192.168.1.135')

length=${#addrs[@]}

for ((k=0;k<$length;k++))
do
 expect expectmain.sh ${targets[$k]} ${addrs[$k]} 'embed'
done

