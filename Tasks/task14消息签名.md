
# 🗂 消息签名与验证设计方案（SM2）

## 1️⃣ 设计目标

1. **身份验证**：确保消息发送者就是其声称的节点
2. **不可否认性**：发送后不能被否认，消息不可篡改
3. **防伪造**：恶意节点无法冒充其他节点发送消息
4. **统一规范**：所有类型消息都必须签名
5. **支持自动验证**：接收节点在接收消息时立即验签

---

## 2️⃣ 消息签名流程

每条消息在发送前，节点都必须执行以下步骤：

### 2.1 生成消息摘要

```text
Digest = Hash(消息内容 + 消息类型 + 时间戳 + 发送者ID)
```

* 消息内容 = message body
* 消息类型 = Task、Reputation、Accuse、Mail、Log 等
* 时间戳 = 精确到秒或毫秒
* 发送者ID = SM2公钥
* Hash = SHA256 或 SM3

### 2.2 SM2 签名

```text
Signature = SM2_Sign(发送者私钥, Digest)
```

* 使用节点的 **SM2私钥**
* 得到签名 `Signature`

### 2.3 附加签名到消息

```json
{
  "Sender": "NodeID_A",
  "MessageType": "TaskSubmit",
  "Timestamp": 1670000000,
  "Content": {...},
  "Signature": "xxx"
}
```

* 消息发送前必须附加 `Signature`
* 发送节点保留原始消息副本，便于调试和日志

---

## 3️⃣ 消息验签流程（接收节点）

接收节点收到消息时执行：

1. **提取发送者ID（公钥）**
2. **计算消息摘要**：

```text
Digest = Hash(Content + MessageType + Timestamp + Sender)
```

3. **使用公钥验证签名**：

```text
Valid = SM2_Verify(SenderPubKey, Digest, Signature)
```

* 如果 `Valid = false` → 拒绝消息并记录日志
* 如果 `Valid = true` → 处理消息

---

## 4️⃣ 应用场景

| 场景   | 消息类型                                 | 验签重要性         |
| ---- | ------------------------------------ | ------------- |
| 任务分发 | TaskSubmit、TaskAssign、TaskResult     | 防止伪造任务或提交     |
| 声誉   | ReputationChange、ReputationPropagate | 防止滥发或伪造声誉     |
| 指责   | Accuse、AccusePropagate               | 确保指责不可否认      |
| 邮箱   | MailSend、MailFetch                   | 保证消息来源真实      |
| 日志   | LogSubmit                            | 防止日志篡改        |
| 网络管理 | NodeJoin、NodeVote                    | 防止伪造节点加入或恶意投票 |

---

## 5️⃣ 补充设计要点

1. **时间戳防重放**：

   * 验证消息时间戳
   * 超过一定时间阈值（如10分钟） → 拒绝消息

2. **消息ID唯一性**：

   * MessageID = Hash(Sender + Timestamp + Content)
   * 避免重复或伪造消息

3. **统一接口**：

   * 所有 agent HTTP/REST接口必须检查签名
   * 发送时自动签名，接收时自动验签

4. **与日志结合**：

   * 验签结果、消息ID、发送者、时间戳 → 日志记录
   * 方便网络异常或安全事件追溯

5. **性能优化**：

   * SM2 签名验签速度较快，适合 P2P 网络
   * 可以批量验证消息，提高效率

---

## 6️⃣ 消息传输示意流程

```
发送节点 (Node A)
   │
   ├─> 生成Digest(Hash(Content+Type+Timestamp+Sender))
   ├─> 使用SM2私钥签名 → Signature
   └─> 发送消息(包含Signature) → 网络

接收节点 (Node B)
   │
   ├─> 提取Sender公钥
   ├─> 重新计算Digest
   ├─> SM2_Verify(Digest, Signature)
   │       │
   │       ├─> 验证成功 → 处理消息
   │       └─> 验证失败 → 拒绝并记录日志
```

---

✅ **总结**

* **统一要求**：所有网络消息必须签名和验签
* **保障安全**：防止伪造、不可否认、防篡改
* **结合现有机制**：任务、声誉、指责、消息、日志
* **增强可追溯性**：日志记录验签结果

