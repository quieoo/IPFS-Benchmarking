#!/bin/bash

# Check if there is an input parameter
if [ $# -eq 0 ]; then
echo "Usage: $0 <max_loss_rate>"
exit 1
fi

# If the parameter is 0, clear the previous packet loss rate configuration
if [ $1 -eq 0 ]; then
echo "Clearing previous packet loss settings"
sudo tc qdisc del dev eth0 root netem
exit 0
fi

# Get the maximum packet loss rate entered by the user
max_loss_rate=$1

# Generate a random packet loss rate between 0 and max_loss_rate
loss_rate=$(( RANDOM % (max_loss_rate + 1) ))

echo "Setting packet loss rate to ${loss_rate}%"

# Use the tc command to configure the packet loss rate of the eth0 interface
sudo tc qdisc add dev eth0 root netem loss ${loss_rate}%

# Display the current configuration
sudo tc qdisc show dev eth0