<!--
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-15 16:58:41
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-15 17:13:58
 * @FilePath: /examples/.kiro/specs/load-testing-tool/tasks.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
# Implementation Plan

- [x] 1. 修改 main.go 添加负载测试功能
  - [x] 1.1 添加配置变量和指标计数器
    - 添加 maxRobotNum, batchSize, batchInterval, errorThreshold 配置
    - 添加 atomic 计数器：onlineCount, totalRequests, successCount, errorCount, totalLatencyMs, maxLatencyMs
    - _Requirements: 1.1, 1.2_
  - [x] 1.2 修改 RunRobot 函数记录指标
    - 在请求前后记录时间戳计算延迟
    - 使用 atomic 更新成功/错误计数
    - 登录成功后增加 onlineCount
    - _Requirements: 2.1, 2.2, 3.1_
  - [x] 1.3 实现批量启动逻辑
    - 按 batchSize 分批启动机器人
    - 每批之间等待 batchInterval
    - 检查错误率，超过阈值停止启动
    - _Requirements: 1.1, 1.2, 1.3, 3.3_
  - [x] 1.4 实现实时状态打印
    - 每 5 秒打印：在线数、平均延迟、错误率、请求数
    - _Requirements: 4.1, 4.2_
  - [x] 1.5 实现最终汇总输出
    - 打印最大在线数、总请求数、成功率、平均/最大延迟、总错误数
    - _Requirements: 5.1, 5.2, 5.3_

- [x] 2. 测试验证
  - [x] 2.1 小规模测试（10个机器人）验证功能正常
    - 确认指标收集正确
    - 确认输出格式正确
  - [x] 2.2 中等规模测试（100个机器人）验证稳定性
    - 观察错误率变化
    - 观察延迟变化

