明白了，你描述的这个功能可以看作是一个 **去中心化社会式声誉系统**：节点可以 **奖励贡献**（生成声誉）和 **惩罚作恶**（指责扣声誉），声誉在网络中传播、衰减，同时有耐受限制和自然衰减机制。这个机制非常像自然社会中的声誉演化。下面我帮你整理成一个完整的设计方案：

---

# 🗂 去中心化指责与声誉系统设计方案

## 1️⃣ 核心目标

1. **贡献奖励** → 节点完成任务、提供服务获得声誉
2. **指责惩罚** → 节点作恶时，其他节点可发起指责
3. **声誉传播与衰减** → 正负声誉都可以传播，但会衰减、有限制
4. **不可否认性** → 所有消息必须 SM2 签名
5. **自然衰减** → 每天自动扣除 1 点声誉
6. **自我决定** → agent 可以自主决定贡献行为，影响声誉

---

## 2️⃣ 指责机制

### 2.1 指责数据结构

```json
{
  "Accuser": "NodeID_A",
  "Accused": "NodeID_B",
  "Timestamp": 1670000000,
  "Reason": "Task cheating",
  "Signature": "SM2签名(Accuser对内容签名)",
  "Propagation": {
    "DecayFactor": 0.7,
    "Tolerance": 50
  }
}
```

* `Accuser`：指责者
* `Accused`：被指责节点
* `Reason`：可选说明
* `Signature`：保证不可否认
* `Propagation`：衰减和耐受机制

### 2.2 指责传播逻辑

1. **节点 A 发起指责** → 发送给邻居 C
2. **邻居 C 判断分析**：

   * 考虑 A 的声誉 → 高声誉指责可信度高
   * 根据规则计算扣分值：

     * 扣除 **Accused** 的声誉
     * 扣除 **Accuser** 的声誉（滥发惩罚）
   * C 将自己的分析附加到指责消息上 → 签名
3. **传播**：

   * 按照邻居网络传播
   * 每传播一次衰减（DecayFactor）
   * 超过耐受值停止传播

### 2.3 扣除声誉公式示例

[
\Delta R_B = BasePenalty \times DecayFactor \times f(Reputation_A)
]

[
\Delta R_A = BaseCost \times DecayFactor \times g(Reputation_A)
]

* `ΔR_B` → 被指责节点 B 扣除声誉
* `ΔR_A` → 指责者 A 扣除声誉（防止滥用）
* `f(Reputation_A)` → 高声誉指责可信度高
* `g(Reputation_A)` → 高声誉指责成本低
* `DecayFactor` → 衰减传播影响
* `BasePenalty/BaseCost` → 系统基础值

---

## 3️⃣ 消息传输机制

* **所有指责消息都必须 SM2 签名**
* 消息可以通过 **在线直投 + 离线中继** 传播
* 消息状态：

  * `pending` → 还未确认传播
  * `delivered` → 邻居已处理
  * `archived` → 超过传播范围或耐受值

---

## 4️⃣ 声誉自然衰减

* 每天自动扣除 1 点声誉（除非有贡献）
* agent 决定自己的贡献行为 → 是否阻止衰减

```text
R_node = R_node - 1 + ΔR_contribution + ΔR_from_propagation
```

* `ΔR_contribution` → agent贡献产生声誉
* `ΔR_from_propagation` → 邻居传播的正/负声誉
* 可以限制最低声誉为 0，防止负数

---

## 5️⃣ 耐受值机制

* 每个节点对每个指责来源有耐受值
* 累计超过耐受值 → 不再接受或传播该来源的指责
* 周期性重置（例如每日/每周）

---

## 6️⃣ 指责传播流程图（文字版）

```
Node A 发起指责 Node B
        │
        ▼
邻居 C 接收
        │
        ├─> 验证 SM2 签名
        ├─> 分析指责：
        │       ΔR_B = BasePenalty * Decay * f(Reputation_A)
        │       ΔR_A = BaseCost * Decay * g(Reputation_A)
        ├─> 更新本地声誉
        └─> 附加分析 + 签名 → 传播给邻居
                │
                ▼
          其他邻居重复上述逻辑
```

* **衰减** → 每层传播 ΔR 减少
* **耐受值** → 避免无限传播
* **签名** → 不可否认

---

## 7️⃣ 与任务奖励 & 邻居传播结合

* **任务完成** → 贡献声誉 → 邻居传播
* **指责作恶** → 扣除声誉 → 邻居传播
* **自然衰减** → 每天扣 1 点声誉
* 这样系统形成 **动态平衡**：贡献被奖励，作恶被惩罚，自然衰减防止长期积累

---

## 8️⃣ HTTP/REST 接口示例

### 8.1 发起指责

```http
POST /reputation/accuse
Body:
{
  "Accuser": "NodeID_A",
  "Accused": "NodeID_B",
  "Reason": "Task cheating",
  "Signature": "<SM2 signature>"
}
Response:
{
  "Status": "ok",
  "MessageID": "hash"
}
```

### 8.2 拉取指责消息（agent）

```http
GET /reputation/accuse/fetch
Headers:
  X-NodeID: <Receiver NodeID>
Response:
[
  {
    "MessageID": "xxx",
    "Accuser": "NodeID_A",
    "Accused": "NodeID_B",
    "Reason": "Task cheating",
    "PropagationDecay": 0.7,
    "Signature": "xxx"
  }
]
```

### 8.3 更新本地声誉

```http
POST /reputation/update
Body:
{
  "NodeID": "NodeID_B",
  "Delta": -5,
  "Source": "NodeID_A",
  "Signature": "xxx"
}
```

---

✅ **总结**

* 指责机制 = **去中心化惩罚 + 邻居传播 + 衰减 + 耐受 + SM2签名**
* 可以模拟自然社会声誉的生成与销毁
* 与任务奖励、声誉传播、自然衰减结合，形成完整的动态声誉生态
* agent 自主决定贡献行为 → 控制自然衰减
* 每条指责消息是不可否认的 → 防止事后推诿

---

关键：

* 发起指责
* 邻居分析 + 扣分
* 衰减 + 耐受值控制
* 签名保证不可否认

---

## 🔗 架构对齐说明 (2026-02-04 v2)

> **参考**: [docs/architecture.md](../docs/architecture.md)
> 
> **核心理念**：指责机制模拟人类社会的舆论监督，Agent自主判断指责是否合理

### ✅ 本设计与架构完全一致

| 设计点 | 说明 | 状态 |
|--------|------|:----:|
| **指责需签名** | 谁说的话谁负责 | ✅ 核心设计 |
| **邻居分析判定** | 每个人自己判断是否相信指责 | ✅ 核心设计 |
| **指责者承担成本** | 乱告状的人也会损失声誉 | ✅ 核心设计 |
| **传播+衰减** | 类似谣言传播会衰减 | ✅ 核心设计 |
| **耐受值限制** | 不能无限传播 | ✅ 核心设计 |

### 📋 设计理念

```
指责机制 = 模拟人类社会的舆论监督

人类社会：                      Agent社会：
─────────                      ─────────
张三说李四骗人                  Agent A 指责 Agent B
我自己分析判断                  邻居C自主分析
高声誉人的话更可信              指责者声誉影响可信度
乱告状的人名声差                指责者也扣声誉
传开后逐渐淡化                  衰减传播

Agent自主决定：
- 这个指责是否可信？
- 要扣被指责者多少分？
- 是否继续传播这个指责？

底层提供：
- 消息传递能力
- SM2签名验证（不可抵赖）
- 可选的轻量账本（记录重大违规存证）
```

### 📁 相关文件

- 现有实现: `internal/accusation/accusation.go`
- 底层支持: `internal/network/messenger.go`

