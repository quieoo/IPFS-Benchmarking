#!/bin/bash

# Parameters: monitoring time, sampling interval, program name
monitor_duration=$1
interval=$2
program_name=$3

# Initialize variables
cpu_total=0
memory_total=0
count=0

echo "Start monitoring program: ${program_name}, monitoring time: ${monitor_duration} seconds, sampling every ${interval} seconds..."

# Get the PID of the program
pid=$(pgrep -x "$program_name")

if [ -z "$pid" ]; then
echo "Program not found: ${program_name}"
exit 1
fi

# Get the current time
start_time=$(date +%s)

# Continue to collect data until the monitoring time ends
while [ $(($(date +%s) - start_time)) -lt "$monitor_duration" ]
do
# Use ps to get the CPU of the specified process and memory usage
ps_output=$(ps -p $pid -o %cpu,%mem --no-headers)

# Check if the process is still running
if [ -z "$ps_output" ];then
echo "Program terminated: ${program_name}"
break
fi

# Parse CPU usage (%cpu) and memory usage (%mem)
cpu_usage=$(echo $ps_output | awk '{print $1}')
mem_usage=$(echo $ps_output | awk '{print $2}')

# Accumulate CPU and memory usage
cpu_total=$(echo "$cpu_total + $cpu_usage" | bc)
memory_total=$(echo "$memory_total + $mem_usage" | bc)
count=$((count + 1))

# Print the current CPU and memory usage
echo "Program: $program_name, CPU utilization: ${cpu_usage}%, memory utilization: ${mem_usage}%"

# Wait for the next sampling
sleep "$interval"
done

# Prevent the sampling number from being 0 and causing a division by 0 error
if [ $count -gt 0 ]; then
# Calculate the average value
avg_cpu=$(echo "$cpu_total / $count" | bc -l)
avg_mem=$(echo "$memory_total / $count" | bc -l)

# Print the result
echo
echo "Monitoring ended, a total of ${count} samples"
echo "Program: $program_name, average CPU utilization: ${avg_cpu}%"
echo "Program: $program_name, average memory utilization: ${avg_mem}%"
else
echo "Not enough sampling data"
fi