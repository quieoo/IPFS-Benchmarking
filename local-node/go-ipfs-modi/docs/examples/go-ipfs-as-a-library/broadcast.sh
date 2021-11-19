#!/bin/bash



targets=('embed78' 'embed67' 'embed66' 'embed79' 'embed61' 'embed62' 'embed64' 'embed65')
addrs=('192.168.1.141' '192.168.1.117' '192.168.1.131' '192.168.1.119' '192.168.1.186' '192.168.1.115' '192.168.1.162' '192.168.1.179')

length=${#targets[@]}

for ((k=0;k<$length;k++))
do
 expect expect.sh ${targets[$k]} ${addrs[$k]} 'embed'
done

