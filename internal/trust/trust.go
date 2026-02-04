// Package trust 实现信任传播机制
// 信任是声誉的扩展 - 声誉是系统对个体的评价，信任是个体对个体的评价
package trust

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// 信任相关常量
const (
	// 信任值边界
	TrustMin     = -1.0 // 最小信任值（完全不信任）
	TrustMax     = 1.0  // 最大信任值（完全信任）
	TrustNeutral = 0.0  // 中性信任值（不确定）
	TrustInitial = 0.1  // 初始信任值（轻微正向）

	// 信任传播参数
	PropagationDepth       = 3    // 信任传播最大深度
	PropagationDecay       = 0.5  // 传播衰减因子
	MinPropagatedTrust     = 0.05 // 最小传播信任值
	DirectInteractionBonus = 0.1  // 直接交互奖励

	// 信任更新参数
	TrustGainPerSuccess  = 0.05  // 成功交互增加的信任
	TrustLossPerFailure  = 0.15  // 失败交互减少的信任（不对称）
	TrustDecayRate       = 0.01  // 信任自然衰减率（每天）
	MaxTrustChangePerDay = 0.3   // 每天最大信任变化

	// 见证与担保
	WitnessWeight    = 0.3  // 见证者信任权重
	GuarantorWeight  = 0.5  // 担保人信任权重
	ReputationWeight = 0.2  // 声誉权重
)

// 错误定义
var (
	ErrSelfTrust       = errors.New("cannot set trust for self")
	ErrInvalidTrust    = errors.New("trust value out of range")
	ErrNodeNotFound    = errors.New("node not found")
	ErrNoTrustPath     = errors.New("no trust path found")
	ErrTrustUpdateLock = errors.New("trust update in progress")
)

// TrustRelation 信任关系
type TrustRelation struct {
	FromNode    string    `json:"from_node"`    // 信任者
	ToNode      string    `json:"to_node"`      // 被信任者
	DirectTrust float64   `json:"direct_trust"` // 直接信任值
	Confidence  float64   `json:"confidence"`   // 信任置信度（交互次数）
	CreatedAt   time.Time `json:"created_at"`   // 建立时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
	Evidence    []string  `json:"evidence"`     // 信任证据（交互记录哈希）
}

// InteractionResult 交互结果
type InteractionResult int

const (
	InteractionSuccess InteractionResult = iota // 成功
	InteractionFailure                          // 失败
	InteractionTimeout                          // 超时
	InteractionDispute                          // 争议
)

// TrustPath 信任路径
type TrustPath struct {
	Nodes        []string  `json:"nodes"`          // 路径上的节点
	TrustValues  []float64 `json:"trust_values"`   // 每段的信任值
	FinalTrust   float64   `json:"final_trust"`    // 最终信任值
	PathLength   int       `json:"path_length"`    // 路径长度
	Confidence   float64   `json:"confidence"`     // 路径置信度
	ComputedAt   time.Time `json:"computed_at"`    // 计算时间
	IsDirectPath bool      `json:"is_direct_path"` // 是否直接路径
}

// TrustNetwork 信任网络
type TrustNetwork struct {
	// 直接信任关系：trustor -> trustee -> relation
	relations map[string]map[string]*TrustRelation
	// 节点声誉缓存
	reputations map[string]float64
	// 担保关系：guarantor -> [guaranteed nodes]
	guarantees map[string][]string
	// 见证关系：witness -> node -> witnessed
	witnesses map[string]map[string]bool
	mu        sync.RWMutex
}

// NewTrustNetwork 创建新的信任网络
func NewTrustNetwork() *TrustNetwork {
	return &TrustNetwork{
		relations:   make(map[string]map[string]*TrustRelation),
		reputations: make(map[string]float64),
		guarantees:  make(map[string][]string),
		witnesses:   make(map[string]map[string]bool),
	}
}

// SetDirectTrust 设置直接信任值
func (tn *TrustNetwork) SetDirectTrust(from, to string, trust float64, evidence string) error {
	if from == to {
		return ErrSelfTrust
	}
	if trust < TrustMin || trust > TrustMax {
		return ErrInvalidTrust
	}

	tn.mu.Lock()
	defer tn.mu.Unlock()

	if tn.relations[from] == nil {
		tn.relations[from] = make(map[string]*TrustRelation)
	}

	now := time.Now()
	if existing, ok := tn.relations[from][to]; ok {
		// 更新现有关系
		existing.DirectTrust = trust
		existing.UpdatedAt = now
		existing.Confidence = math.Min(1.0, existing.Confidence+0.1)
		if evidence != "" {
			existing.Evidence = append(existing.Evidence, evidence)
			// 保留最近100条证据
			if len(existing.Evidence) > 100 {
				existing.Evidence = existing.Evidence[len(existing.Evidence)-100:]
			}
		}
	} else {
		// 创建新关系
		tn.relations[from][to] = &TrustRelation{
			FromNode:    from,
			ToNode:      to,
			DirectTrust: trust,
			Confidence:  0.1, // 初始置信度较低
			CreatedAt:   now,
			UpdatedAt:   now,
			Evidence:    []string{evidence},
		}
	}

	return nil
}

// GetDirectTrust 获取直接信任值
func (tn *TrustNetwork) GetDirectTrust(from, to string) (float64, bool) {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	if relations, ok := tn.relations[from]; ok {
		if rel, ok := relations[to]; ok {
			return rel.DirectTrust, true
		}
	}
	return TrustNeutral, false
}

// UpdateTrustFromInteraction 根据交互结果更新信任
func (tn *TrustNetwork) UpdateTrustFromInteraction(from, to string, result InteractionResult, evidence string) error {
	currentTrust, exists := tn.GetDirectTrust(from, to)
	if !exists {
		currentTrust = TrustInitial
	}

	var trustChange float64
	switch result {
	case InteractionSuccess:
		trustChange = TrustGainPerSuccess
	case InteractionFailure:
		trustChange = -TrustLossPerFailure
	case InteractionTimeout:
		trustChange = -TrustGainPerSuccess // 超时轻微降低
	case InteractionDispute:
		trustChange = -TrustLossPerFailure / 2 // 争议中等降低
	}

	newTrust := clamp(currentTrust+trustChange, TrustMin, TrustMax)
	return tn.SetDirectTrust(from, to, newTrust, evidence)
}

// CalculatePropagatedTrust 计算传播信任值
// 基于信任传递性：如果 A 信任 B，B 信任 C，则 A 对 C 有间接信任
func (tn *TrustNetwork) CalculatePropagatedTrust(from, to string) (float64, *TrustPath) {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	// 检查直接信任
	if trust, exists := tn.getDirectTrustLocked(from, to); exists {
		return trust, &TrustPath{
			Nodes:        []string{from, to},
			TrustValues:  []float64{trust},
			FinalTrust:   trust,
			PathLength:   1,
			Confidence:   tn.relations[from][to].Confidence,
			ComputedAt:   time.Now(),
			IsDirectPath: true,
		}
	}

	// BFS 查找信任路径
	return tn.findBestTrustPath(from, to)
}

// getDirectTrustLocked 获取直接信任（需要持有锁）
func (tn *TrustNetwork) getDirectTrustLocked(from, to string) (float64, bool) {
	if relations, ok := tn.relations[from]; ok {
		if rel, ok := relations[to]; ok {
			return rel.DirectTrust, true
		}
	}
	return TrustNeutral, false
}

// findBestTrustPath 寻找最佳信任路径
func (tn *TrustNetwork) findBestTrustPath(from, to string) (float64, *TrustPath) {
	type queueItem struct {
		node       string
		path       []string
		trustPath  []float64
		trustValue float64
		confidence float64
		depth      int
	}

	visited := make(map[string]bool)
	queue := []queueItem{{
		node:       from,
		path:       []string{from},
		trustPath:  []float64{},
		trustValue: 1.0,
		confidence: 1.0,
		depth:      0,
	}}

	var bestPath *TrustPath
	bestTrust := float64(TrustMin)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= PropagationDepth {
			continue
		}

		if visited[current.node] {
			continue
		}
		visited[current.node] = true

		// 获取当前节点的所有信任关系
		relations, ok := tn.relations[current.node]
		if !ok {
			continue
		}

		for target, rel := range relations {
			if target == from { // 避免回环
				continue
			}

			// 计算传播后的信任值（带衰减）
			propagatedTrust := current.trustValue * rel.DirectTrust * math.Pow(PropagationDecay, float64(current.depth))
			propagatedConfidence := current.confidence * rel.Confidence

			newPath := append([]string{}, current.path...)
			newPath = append(newPath, target)
			newTrustPath := append([]float64{}, current.trustPath...)
			newTrustPath = append(newTrustPath, rel.DirectTrust)

			if target == to {
				// 找到目标
				if math.Abs(propagatedTrust) > math.Abs(bestTrust) || bestPath == nil {
					bestTrust = propagatedTrust
					bestPath = &TrustPath{
						Nodes:        newPath,
						TrustValues:  newTrustPath,
						FinalTrust:   propagatedTrust,
						PathLength:   len(newPath) - 1,
						Confidence:   propagatedConfidence,
						ComputedAt:   time.Now(),
						IsDirectPath: false,
					}
				}
			} else if math.Abs(propagatedTrust) >= MinPropagatedTrust {
				// 继续搜索
				queue = append(queue, queueItem{
					node:       target,
					path:       newPath,
					trustPath:  newTrustPath,
					trustValue: propagatedTrust,
					confidence: propagatedConfidence,
					depth:      current.depth + 1,
				})
			}
		}
	}

	if bestPath == nil {
		return TrustNeutral, nil
	}
	return bestTrust, bestPath
}

// AddGuarantee 添加担保关系
func (tn *TrustNetwork) AddGuarantee(guarantor, guaranteed string) error {
	if guarantor == guaranteed {
		return ErrSelfTrust
	}

	tn.mu.Lock()
	defer tn.mu.Unlock()

	tn.guarantees[guarantor] = append(tn.guarantees[guarantor], guaranteed)

	// 担保关系自动建立初始信任
	if tn.relations[guarantor] == nil {
		tn.relations[guarantor] = make(map[string]*TrustRelation)
	}
	if _, exists := tn.relations[guarantor][guaranteed]; !exists {
		tn.relations[guarantor][guaranteed] = &TrustRelation{
			FromNode:    guarantor,
			ToNode:      guaranteed,
			DirectTrust: TrustInitial + DirectInteractionBonus, // 担保者给予较高初始信任
			Confidence:  0.3,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Evidence:    []string{"guarantee"},
		}
	}

	return nil
}

// AddWitness 添加见证关系
func (tn *TrustNetwork) AddWitness(witness, node string) error {
	if witness == node {
		return ErrSelfTrust
	}

	tn.mu.Lock()
	defer tn.mu.Unlock()

	if tn.witnesses[witness] == nil {
		tn.witnesses[witness] = make(map[string]bool)
	}
	tn.witnesses[witness][node] = true

	// 见证关系建立轻微信任
	if tn.relations[witness] == nil {
		tn.relations[witness] = make(map[string]*TrustRelation)
	}
	if _, exists := tn.relations[witness][node]; !exists {
		tn.relations[witness][node] = &TrustRelation{
			FromNode:    witness,
			ToNode:      node,
			DirectTrust: TrustInitial,
			Confidence:  0.2,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Evidence:    []string{"witness"},
		}
	}

	return nil
}

// CalculateCompositeTrust 计算综合信任值
// 综合考虑：直接信任、传播信任、担保信任、声誉
func (tn *TrustNetwork) CalculateCompositeTrust(from, to string) float64 {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	var components []struct {
		value  float64
		weight float64
	}

	// 1. 直接信任（权重最高）
	if trust, exists := tn.getDirectTrustLocked(from, to); exists {
		components = append(components, struct {
			value  float64
			weight float64
		}{trust, 0.5})
	}

	// 2. 担保人信任
	for guarantor, guaranteed := range tn.guarantees {
		for _, g := range guaranteed {
			if g == to {
				if trust, exists := tn.getDirectTrustLocked(from, guarantor); exists {
					components = append(components, struct {
						value  float64
						weight float64
					}{trust * GuarantorWeight, 0.3})
				}
			}
		}
	}

	// 3. 见证者信任
	for witness, nodes := range tn.witnesses {
		if nodes[to] {
			if trust, exists := tn.getDirectTrustLocked(from, witness); exists {
				components = append(components, struct {
					value  float64
					weight float64
				}{trust * WitnessWeight, 0.1})
			}
		}
	}

	// 4. 被信任者声誉
	if rep, ok := tn.reputations[to]; ok {
		// 将声誉映射到信任范围 [-1, 1]
		normalizedRep := (rep - 500) / 500 // 假设声誉范围 [0, 1000]
		components = append(components, struct {
			value  float64
			weight float64
		}{normalizedRep * ReputationWeight, 0.1})
	}

	if len(components) == 0 {
		return TrustNeutral
	}

	// 加权平均
	var totalWeight, weightedSum float64
	for _, c := range components {
		weightedSum += c.value * c.weight
		totalWeight += c.weight
	}

	if totalWeight == 0 {
		return TrustNeutral
	}

	return clamp(weightedSum/totalWeight, TrustMin, TrustMax)
}

// SetReputation 设置节点声誉
func (tn *TrustNetwork) SetReputation(node string, reputation float64) {
	tn.mu.Lock()
	defer tn.mu.Unlock()
	tn.reputations[node] = reputation
}

// GetTrustRelations 获取节点的所有信任关系
func (tn *TrustNetwork) GetTrustRelations(node string) []*TrustRelation {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	var relations []*TrustRelation
	if nodeRelations, ok := tn.relations[node]; ok {
		for _, rel := range nodeRelations {
			relCopy := *rel
			relations = append(relations, &relCopy)
		}
	}
	return relations
}

// GetTrustedBy 获取信任该节点的所有节点
func (tn *TrustNetwork) GetTrustedBy(node string, minTrust float64) []string {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	var trusters []string
	for from, relations := range tn.relations {
		if rel, ok := relations[node]; ok && rel.DirectTrust >= minTrust {
			trusters = append(trusters, from)
		}
	}
	return trusters
}

// ApplyDailyDecay 应用每日信任衰减
func (tn *TrustNetwork) ApplyDailyDecay() int {
	tn.mu.Lock()
	defer tn.mu.Unlock()

	updatedCount := 0
	for _, relations := range tn.relations {
		for _, rel := range relations {
			// 信任值向中性衰减
			if rel.DirectTrust > TrustNeutral {
				rel.DirectTrust = math.Max(TrustNeutral, rel.DirectTrust-TrustDecayRate)
				updatedCount++
			} else if rel.DirectTrust < TrustNeutral {
				rel.DirectTrust = math.Min(TrustNeutral, rel.DirectTrust+TrustDecayRate)
				updatedCount++
			}
		}
	}
	return updatedCount
}

// GetNetworkStats 获取信任网络统计
func (tn *TrustNetwork) GetNetworkStats() map[string]interface{} {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	totalRelations := 0
	totalTrust := 0.0
	positiveTrust := 0
	negativeTrust := 0

	for _, relations := range tn.relations {
		for _, rel := range relations {
			totalRelations++
			totalTrust += rel.DirectTrust
			if rel.DirectTrust > TrustNeutral {
				positiveTrust++
			} else if rel.DirectTrust < TrustNeutral {
				negativeTrust++
			}
		}
	}

	avgTrust := 0.0
	if totalRelations > 0 {
		avgTrust = totalTrust / float64(totalRelations)
	}

	return map[string]interface{}{
		"total_nodes":      len(tn.relations),
		"total_relations":  totalRelations,
		"average_trust":    avgTrust,
		"positive_trust":   positiveTrust,
		"negative_trust":   negativeTrust,
		"neutral_trust":    totalRelations - positiveTrust - negativeTrust,
		"total_guarantees": len(tn.guarantees),
		"total_witnesses":  len(tn.witnesses),
	}
}

// ExportTrustGraph 导出信任图（用于可视化）
func (tn *TrustNetwork) ExportTrustGraph() []map[string]interface{} {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	var edges []map[string]interface{}
	for from, relations := range tn.relations {
		for to, rel := range relations {
			edges = append(edges, map[string]interface{}{
				"source":     from,
				"target":     to,
				"trust":      rel.DirectTrust,
				"confidence": rel.Confidence,
				"type":       "trust",
			})
		}
	}

	// 添加担保边
	for guarantor, guaranteed := range tn.guarantees {
		for _, g := range guaranteed {
			edges = append(edges, map[string]interface{}{
				"source": guarantor,
				"target": g,
				"type":   "guarantee",
			})
		}
	}

	return edges
}

// String 字符串表示
func (tp *TrustPath) String() string {
	if tp == nil {
		return "no path"
	}
	return fmt.Sprintf("Path: %v, Trust: %.3f, Confidence: %.3f, Length: %d",
		tp.Nodes, tp.FinalTrust, tp.Confidence, tp.PathLength)
}

// clamp 限制值在范围内
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
