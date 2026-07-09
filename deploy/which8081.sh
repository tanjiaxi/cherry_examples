# 客户端Mac上执行（监控客户端瓶颈）
while true; do
  echo "=== $(date) ==="
  # 1. 查看当前建立中的连接数（SYN_SENT状态）
  netstat -an | grep 10.10.10.251:8081 | awk '{print $6}' | sort | uniq -c

  # 2. 查看客户端进程的FD使用情况
  lsof -p <robot_client_pid> | wc -l

  # 3. 查看系统临时端口使用情况
  sysctl net.inet.ip.portrange.first net.inet.ip.portrange.last
  netstat -an | grep ESTABLISHED | wc -l

  sleep 2
done
