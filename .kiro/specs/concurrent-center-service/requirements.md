<!--
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-25 10:46:35
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-25 11:56:16
 * @FilePath: /examples/.kiro/specs/concurrent-center-service/requirements.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
# Requirements Document

## Introduction

本文档定义了 Cherry 框架中"并发公共服务组件"的需求。该组件旨在解决 center 节点使用单一 Actor 模型导致的并发瓶颈问题，为无状态的公共服务（如登录验证、账号查询）提供高并发处理能力。

## 问题分析

当前架构问题：
- center 节点的 `ActorAccount` 是单一 Actor，所有 RPC 请求进入同一个 mailbox 队列
- 500 并发请求 × 30ms/请求 = 15 秒串行处理时间
- NATS cluster 的 `remoteProcess()` 将消息投递到 `ActorSystem().PostRemote()`，进入 Actor 队列

## 解决方案：方案 C - 自定义 Cluster 组件

**核心思路：**
- 创建自定义的 Cluster 组件，继承原有 nats_cluster 的功能
- 在 `remoteProcess()` 中根据目标 Actor 类型决定处理方式：
  - 并发服务：直接开 goroutine 处理，不进入 Actor mailbox
  - 普通 Actor：保持原有逻辑，投递到 mailbox 串行处理
- 调用方代码完全不变，只需在 center 节点替换 Cluster 组件

**优点：**
1. 调用方代码（gate/web）完全不变
2. 不修改 vendor 代码，通过 `app.SetCluster()` 替换
3. 与现有 Actor 系统共存，可以灵活配置哪些服务用并发模式
4. 升级 cherry 框架无冲突

## Glossary

- **Concurrent_Cluster**: 并发集群组件，继承原有 nats_cluster，在 remoteProcess 中支持并发处理
- **Concurrent_Service**: 并发服务，标记为并发模式的 Actor，请求直接并发处理不进入 mailbox
- **Center_Node**: 中心节点，提供账号验证、UID 绑定等公共服务
- **Actor_Model**: Actor 模型，消息通过 mailbox 队列串行处理，适合有状态服务
- **NATS_Subject**: NATS 消息主题，用于节点间通信
- **Service_Handler**: 服务处理函数，接收 protobuf 请求并返回响应
- **Worker_Pool**: 工作池，管理一组 goroutine 用于并发处理请求，防止 goroutine 爆炸

## Requirements

### Requirement 1: 自定义 Cluster 组件

**User Story:** As a system architect, I want to create a custom Cluster component that supports concurrent processing, so that I can handle high-concurrency login requests without modifying the cherry framework.

#### Acceptance Criteria

1. THE Concurrent_Cluster SHALL inherit all functionality from the original nats_cluster component
2. THE Concurrent_Cluster SHALL be registered via `app.SetCluster()` before `app.Startup()`
3. WHEN a message arrives for a Concurrent_Service, THE Concurrent_Cluster SHALL process it in a goroutine without entering Actor mailbox
4. WHEN a message arrives for a normal Actor, THE Concurrent_Cluster SHALL use the original logic (PostRemote to mailbox)
5. THE Concurrent_Cluster SHALL support configuring which Actors are Concurrent_Services via a list

### Requirement 2: 并发服务注册

**User Story:** As a developer, I want to register service handlers for concurrent processing, so that I can migrate existing Actor-based services easily.

#### Acceptance Criteria

1. THE Concurrent_Cluster SHALL provide a RegisterConcurrent(actorID, methodName, handler) API
2. WHEN a registered concurrent method is called, THE handler SHALL be invoked directly in a goroutine
3. THE handler signature SHALL match the original Actor Remote handler: `func(req *pb.XXX) (*pb.YYY, int32)`
4. THE Concurrent_Cluster SHALL use the same protobuf serializer as the Actor system

### Requirement 3: Worker Pool 控制

**User Story:** As an operator, I want to limit the maximum concurrent goroutines, so that the system doesn't exhaust resources under high load.

#### Acceptance Criteria

1. THE Concurrent_Cluster SHALL support configurable Worker_Pool size (default: 1000)
2. WHEN the Worker_Pool is exhausted, THE Concurrent_Cluster SHALL queue new requests
3. IF a request waits longer than the configured timeout, THEN THE Concurrent_Cluster SHALL return a timeout error
4. THE Worker_Pool size SHALL be configurable via profile configuration

### Requirement 4: 向后兼容

**User Story:** As a developer, I want the concurrent service to be fully backward compatible, so that existing RPC calls work without modification.

#### Acceptance Criteria

1. THE Concurrent_Cluster SHALL use the same NATS subject format as the original cluster
2. WHEN calling from gate/web nodes, THE caller code (internal/rpc/center) SHALL remain unchanged
3. THE Concurrent_Cluster SHALL support both concurrent and Actor-based services on the same node
4. THE response format SHALL be identical to the original Actor response (cproto.Response)
