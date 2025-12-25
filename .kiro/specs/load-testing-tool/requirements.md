# Requirements Document

## Introduction

本文档定义了游戏服务器简化版负载测试工具的需求。该工具基于现有的 robot_client 进行扩展，专注于测试服务器的最大承载能力、稳定性和性能瓶颈。

核心目标：
- 测试最大同时在线人数
- 监控满负载时的响应延迟
- 验证服务器在压力下的稳定性（拒绝服务而非崩溃）
- 识别性能瓶颈

## Glossary

- **Robot**: 模拟玩家的客户端实例
- **Latency**: 请求响应延迟
- **ErrorRate**: 错误率，用于判断服务器是否开始拒绝服务

## Requirements

### Requirement 1

**User Story:** As a developer, I want to gradually increase concurrent users, so that I can find the server's maximum capacity.

#### Acceptance Criteria

1. WHEN the test starts THEN the system SHALL spawn robots in batches (e.g., 10 users per batch)
2. WHEN spawning robots THEN the system SHALL wait for all robots in current batch to connect before spawning next batch
3. WHEN error rate exceeds 10% THEN the system SHALL stop spawning new robots and record current online count as capacity limit

### Requirement 2

**User Story:** As a developer, I want to monitor response latency, so that I can understand server performance under load.

#### Acceptance Criteria

1. WHEN a robot sends a request THEN the system SHALL measure round-trip latency
2. WHEN displaying metrics THEN the system SHALL show average latency for the current period
3. WHEN latency exceeds 1 second THEN the system SHALL flag it as degraded performance

### Requirement 3

**User Story:** As a developer, I want to track success and error rates, so that I can detect when server starts rejecting requests.

#### Acceptance Criteria

1. WHEN requests complete THEN the system SHALL count successes and failures
2. WHEN displaying metrics THEN the system SHALL show current error rate as percentage
3. WHEN error rate increases sharply THEN the system SHALL log it as potential capacity limit reached

### Requirement 4

**User Story:** As a developer, I want real-time console output, so that I can monitor test progress.

#### Acceptance Criteria

1. WHILE the test is running THEN the system SHALL print status every 5 seconds
2. WHEN printing status THEN the system SHALL show: online count, avg latency, error rate, errors count

### Requirement 5

**User Story:** As a developer, I want a final summary, so that I can analyze test results.

#### Acceptance Criteria

1. WHEN the test completes THEN the system SHALL print a summary with max online users achieved
2. WHEN the test completes THEN the system SHALL print average and peak latency
3. WHEN the test completes THEN the system SHALL print total errors and error rate

