package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"

	"redisctl/internal/config"
	"redisctl/internal/styles"
)

// NewRebalanceCommand 'rebalance' 명령어
func NewRebalanceCommand() *cobra.Command {
	var dryRun bool
	var threshold int
	var pipeline int

	cmd := &cobra.Command{
		Use:   "rebalance <cluster-node-ip:port>",
		Short: "r 클러스터의 슬롯 분배를 자동으로 균형 조정합니다",
		Long: styles.TitleStyle.Render("[=] 클러스터 슬롯 자동 균형 조정") + "\n\n" +
			styles.DescStyle.Render("Redis 클러스터의 슬롯 분배를 모든 마스터 노드에 균등하게 재분배합니다.") + "\n" +
			styles.DescStyle.Render("노드 추가/제거 후 불균형한 슬롯 분배를 자동으로 최적화합니다.") + "\n\n" +
			styles.DescStyle.Render("주요 기능:") + "\n" +
			styles.DescStyle.Render("• 현재 슬롯 분배 상태 분석") + "\n" +
			styles.DescStyle.Render("• 최적 분배 계산 및 이동 계획 수립") + "\n" +
			styles.DescStyle.Render("• 안전한 배치 슬롯 이동 (MIGRATE 사용)") + "\n" +
			styles.DescStyle.Render("• 드라이런 모드로 변경사항 미리보기") + "\n" +
			styles.DescStyle.Render("• 임계값 기반 선택적 리밸런싱"),
		Example: `  # 클러스터 자동 리밸런싱
  redisctl rebalance localhost:7001

  # 드라이런 모드로 변경사항 미리보기
  redisctl rebalance --dry-run localhost:7001

  # 10% 이상 불균형시에만 리밸런싱
  redisctl --password mypass rebalance --threshold 10 localhost:9001

  # 파이프라인 크기 조정으로 성능 최적화
  redisctl rebalance --pipeline 20 localhost:7001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ValidateAuth(); err != nil {
				return err
			}
			return runRebalanceCluster(args[0], dryRun, threshold, pipeline)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "실제 변경 없이 리밸런싱 계획만 표시")
	cmd.Flags().IntVar(&threshold, "threshold", 5, "리밸런싱 임계값 (퍼센트, 기본: 5%)")
	cmd.Flags().IntVar(&pipeline, "pipeline", 10, "MIGRATE당 키 수 (기본: 10)")

	return cmd
}

type MasterNode struct {
	ID    string
	Addr  string
	Slots []int
}

type ReplicaNode struct {
	ID       string
	Addr     string
	MasterID string
}

type RebalancePlan struct {
	From      string
	To        string
	Slots     []int
	SlotCount int
}

func runRebalanceCluster(clusterAddr string, dryRun bool, threshold, pipeline int) error {
	fmt.Println(styles.InfoStyle.Render("Redis 클러스터 슬롯 균형 조정"))
	fmt.Printf("클러스터: %s\n", styles.HighlightStyle.Render(clusterAddr))
	if dryRun {
		fmt.Println(styles.WarningStyle.Render("드라이런 모드: 실제 변경 없이 계획만 표시"))
	}
	fmt.Println()

	// Connect to cluster
	user, password := config.GetAuth()
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    []string{clusterAddr},
		Username: user,
		Password: password,
	})
	defer client.Close()

	ctx := context.Background()

	// Validate cluster connectivity
	if err := validateRebalanceConnectivity(ctx, client); err != nil {
		return fmt.Errorf("클러스터 연결 실패: %w", err)
	}

	// Get current slot distribution and cluster topology
	masters, replicas, err := getCurrentClusterTopology(ctx, client)
	if err != nil {
		return fmt.Errorf("클러스터 토폴로지 조회 실패: %w", err)
	}

	// Validate that we have masters
	if len(masters) == 0 {
		return fmt.Errorf("클러스터에 마스터 노드가 없습니다")
	}

	// Additional safety checks
	if err := validateClusterForRebalancing(ctx, client); err != nil {
		return fmt.Errorf("리밸런싱 안전성 검증 실패: %w", err)
	}

	// Check cluster health and provide recommendations
	checkClusterTopology(masters, replicas)

	// Calculate current imbalance
	imbalance := calculateImbalance(masters)
	fmt.Printf("현재 불균형도: %s\n",
		styles.HighlightStyle.Render(fmt.Sprintf("%.1f%%", imbalance)))

	// Check if rebalancing is needed
	if imbalance < float64(threshold) {
		fmt.Println(styles.SuccessStyle.Render("OK 클러스터가 이미 균형잡혀 있습니다!"))
		fmt.Printf("임계값 %d%% 미만이므로 리밸런싱이 필요하지 않습니다.\n", threshold)
		return nil
	}

	// Generate rebalancing plan
	plan := generateRebalancePlan(masters)
	if len(plan) == 0 {
		fmt.Println(styles.SuccessStyle.Render("OK 리밸런싱이 필요하지 않습니다!"))
		return nil
	}

	// Display the plan
	displayRebalancePlan(plan, masters)

	// Execute the plan (if not dry-run)
	if !dryRun {
		if err := executeRebalancePlan(ctx, client, plan, pipeline); err != nil {
			return fmt.Errorf("리밸런싱 실행 실패: %w", err)
		}

		fmt.Println()
		fmt.Println(styles.SuccessStyle.Render("OK 클러스터 리밸런싱이 완료되었습니다!"))

		// Show final distribution
		finalMasters, err := getCurrentSlotDistribution(ctx, client)
		if err == nil {
			finalImbalance := calculateImbalance(finalMasters)
			fmt.Printf("최종 불균형도: %s\n",
				styles.SuccessStyle.Render(fmt.Sprintf("%.1f%%", finalImbalance)))
		}
	} else {
		fmt.Println()
		fmt.Println(styles.InfoStyle.Render("실제 리밸런싱을 수행하려면 --dry-run 플래그를 제거하세요"))
	}

	return nil
}

func validateRebalanceConnectivity(ctx context.Context, client *redis.ClusterClient) error {
	fmt.Print(styles.InfoStyle.Render("1. 클러스터 연결 확인..."))

	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return err
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return nil
}

func validateClusterForRebalancing(ctx context.Context, client *redis.ClusterClient) error {
	fmt.Print(styles.InfoStyle.Render("  클러스터 안전성 검증..."))

	// Check cluster state
	info := client.ClusterInfo(ctx)
	if info.Err() != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return fmt.Errorf("클러스터 정보 조회 실패: %w", info.Err())
	}

	infoStr := info.Val()
	if !strings.Contains(infoStr, "cluster_state:ok") {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return fmt.Errorf("클러스터가 정상 상태가 아닙니다. 리밸런싱 전에 클러스터 문제를 해결하세요")
	}

	// Check for ongoing cluster operations
	nodes := client.ClusterNodes(ctx)
	if nodes.Err() != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return fmt.Errorf("클러스터 노드 조회 실패: %w", nodes.Err())
	}

	// Look for nodes in transitional states
	lines := strings.Split(nodes.Val(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check for handshake, fail, or other problematic states
		if strings.Contains(line, "handshake") || strings.Contains(line, "fail") {
			fmt.Println(styles.ErrorStyle.Render(" 실패"))
			return fmt.Errorf("클러스터에 불안정한 노드가 있습니다. 리밸런싱 전에 해결하세요")
		}

		// Check for ongoing slot migration (migrating/importing states)
		if strings.Contains(line, "migrating") || strings.Contains(line, "importing") {
			fmt.Println(styles.ErrorStyle.Render(" 실패"))
			return fmt.Errorf("진행 중인 슬롯 마이그레이션이 있습니다. 완료 후 다시 시도하세요")
		}
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return nil
}

func getCurrentSlotDistribution(ctx context.Context, client *redis.ClusterClient) ([]MasterNode, error) {
	masters, _, err := getCurrentClusterTopology(ctx, client)
	return masters, err
}

func getCurrentClusterTopology(ctx context.Context, client *redis.ClusterClient) ([]MasterNode, []ReplicaNode, error) {
	fmt.Print(styles.InfoStyle.Render("2. 클러스터 토폴로지 조회..."))

	result := client.ClusterNodes(ctx)
	if result.Err() != nil {
		fmt.Println(styles.ErrorStyle.Render(" 실패"))
		return nil, nil, result.Err()
	}

	var masters []MasterNode
	var replicas []ReplicaNode
	lines := strings.Split(result.Val(), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 8 {
			continue
		}

		// Check node type using unified utility
		nodeFlags := parseNodeFlagsSlice(strings.Split(parts[2], ","))

		if nodeFlags.IsMaster {
			master := MasterNode{
				ID:   parts[0],
				Addr: parts[1],
			}

			// Parse slots (starting from index 8)
			if len(parts) > 8 {
				for i := 8; i < len(parts); i++ {
					slotRange := parts[i]
					if strings.Contains(slotRange, "-") {
						// Range like "0-5460"
						rangeParts := strings.Split(slotRange, "-")
						if len(rangeParts) == 2 {
							start, err1 := strconv.Atoi(rangeParts[0])
							end, err2 := strconv.Atoi(rangeParts[1])
							if err1 == nil && err2 == nil {
								for slot := start; slot <= end; slot++ {
									master.Slots = append(master.Slots, slot)
								}
							}
						}
					} else {
						// Single slot
						if slot, err := strconv.Atoi(slotRange); err == nil {
							master.Slots = append(master.Slots, slot)
						}
					}
				}
			}

			masters = append(masters, master)
		} else if nodeFlags.IsReplica {
			replica := ReplicaNode{
				ID:       parts[0],
				Addr:     parts[1],
				MasterID: parts[3],
			}
			replicas = append(replicas, replica)
		}
	}

	fmt.Println(styles.SuccessStyle.Render(" 완료"))
	return masters, replicas, nil
}

func checkClusterTopology(masters []MasterNode, replicas []ReplicaNode) {
	fmt.Println(styles.InfoStyle.Render("클러스터 토폴로지 분석"))

	masterCount := len(masters)
	replicaCount := len(replicas)

	fmt.Printf("마스터: %s, 레플리카: %s\n",
		styles.HighlightStyle.Render(strconv.Itoa(masterCount)),
		styles.HighlightStyle.Render(strconv.Itoa(replicaCount)))

	// Check for topology issues and provide recommendations
	var warnings []string
	var recommendations []string

	// Check 1: Master count
	if masterCount < 3 {
		warnings = append(warnings, fmt.Sprintf("마스터 수가 권장값(3개)보다 적습니다 (%d개)", masterCount))
		if replicaCount > 0 {
			recommendations = append(recommendations,
				"고가용성을 위해 레플리카를 마스터로 승격시키는 것을 고려하세요:")
			recommendations = append(recommendations,
				"  # 레플리카를 마스터로 승격 (슬롯 할당)")
			for i, replica := range replicas {
				if i >= 3-masterCount { // Only suggest enough to reach 3 masters
					break
				}
				recommendations = append(recommendations,
					fmt.Sprintf("  redisctl reshard --from <마스터ID> --to %s --slots <수량> <클러스터주소>", replica.ID))
			}
		} else {
			recommendations = append(recommendations, "새로운 마스터 노드를 추가하는 것을 고려하세요")
		}
	}

	// Check 2: No replicas
	if replicaCount == 0 {
		warnings = append(warnings, "레플리카가 없어 고가용성이 확보되지 않았습니다")
		recommendations = append(recommendations, "고가용성을 위해 레플리카 노드 추가를 권장합니다:")
		recommendations = append(recommendations, "  redisctl add-node <새노드주소> <클러스터주소> --replica")
	}

	// Check 3: Uneven replica distribution
	if masterCount > 0 && replicaCount > 0 {
		replicaPerMaster := make(map[string]int)
		for _, replica := range replicas {
			replicaPerMaster[replica.MasterID]++
		}

		maxReplicas := 0
		minReplicas := replicaCount
		for _, count := range replicaPerMaster {
			if count > maxReplicas {
				maxReplicas = count
			}
			if count < minReplicas {
				minReplicas = count
			}
		}

		// Some masters have no replicas
		mastersWithoutReplicas := masterCount - len(replicaPerMaster)
		if mastersWithoutReplicas > 0 {
			minReplicas = 0
		}

		if maxReplicas-minReplicas > 1 {
			warnings = append(warnings, "레플리카 분배가 불균등합니다")
			recommendations = append(recommendations, "레플리카 재분배를 고려하세요 (수동 조정 필요)")
		}
	}

	// Display warnings and recommendations
	if len(warnings) > 0 {
		fmt.Println()
		fmt.Println(styles.WarningStyle.Render("  토폴로지 경고:"))
		for i, warning := range warnings {
			fmt.Printf("  %d. %s\n", i+1, warning)
		}
	}

	if len(recommendations) > 0 {
		fmt.Println()
		fmt.Println(styles.InfoStyle.Render(" 권장사항:"))
		for _, rec := range recommendations {
			if strings.HasPrefix(rec, "  ") {
				fmt.Printf("    %s\n", styles.DescStyle.Render(rec))
			} else {
				fmt.Printf("  • %s\n", rec)
			}
		}
	}

	if len(warnings) == 0 {
		fmt.Printf("%s 클러스터 토폴로지가 양호합니다\n", styles.SuccessStyle.Render("OK"))
	}

	fmt.Println()
}

func calculateImbalance(masters []MasterNode) float64 {
	if len(masters) == 0 {
		return 0
	}

	idealSlots := 16384 / len(masters)
	maxDeviation := 0

	for _, master := range masters {
		deviation := len(master.Slots) - idealSlots
		if deviation < 0 {
			deviation = -deviation
		}
		if deviation > maxDeviation {
			maxDeviation = deviation
		}
	}

	return float64(maxDeviation) / float64(idealSlots) * 100
}

func generateRebalancePlan(originalMasters []MasterNode) []RebalancePlan {
	if len(originalMasters) == 0 {
		return nil
	}

	// Create a deep copy of masters to avoid modifying the original
	masters := make([]MasterNode, len(originalMasters))
	for i, master := range originalMasters {
		masters[i] = MasterNode{
			ID:    master.ID,
			Addr:  master.Addr,
			Slots: make([]int, len(master.Slots)),
		}
		copy(masters[i].Slots, master.Slots)
	}

	idealSlots := 16384 / len(masters)
	var plan []RebalancePlan

	// Create a more efficient rebalancing plan
	// Sort masters by slot count to identify donors and receivers clearly
	sort.Slice(masters, func(i, j int) bool {
		return len(masters[i].Slots) > len(masters[j].Slots)
	})

	// Separate donors (excess) and receivers (deficit)
	var donors []int    // indices of masters with excess slots
	var receivers []int // indices of masters with deficit slots

	for i, master := range masters {
		slotCount := len(master.Slots)
		if slotCount > idealSlots {
			donors = append(donors, i)
		} else if slotCount < idealSlots {
			receivers = append(receivers, i)
		}
	}

	// Generate optimal plan by pairing donors with receivers
	donorIdx := 0
	receiverIdx := 0

	for donorIdx < len(donors) && receiverIdx < len(receivers) {
		donor := &masters[donors[donorIdx]]
		receiver := &masters[receivers[receiverIdx]]

		excess := len(donor.Slots) - idealSlots
		deficit := idealSlots - len(receiver.Slots)

		// Move the minimum of excess and deficit
		slotsToMove := excess
		if deficit < excess {
			slotsToMove = deficit
		}

		if slotsToMove <= 0 {
			break
		}

		// Select slots to move (take from the end for better locality)
		slotsToTransfer := make([]int, 0, slotsToMove)
		for i := len(donor.Slots) - 1; i >= 0 && len(slotsToTransfer) < slotsToMove; i-- {
			slotsToTransfer = append(slotsToTransfer, donor.Slots[i])
		}

		// Add to plan
		plan = append(plan, RebalancePlan{
			From:      donor.ID,
			To:        receiver.ID,
			Slots:     slotsToTransfer,
			SlotCount: len(slotsToTransfer),
		})

		// Update the masters for next iteration
		donor.Slots = donor.Slots[:len(donor.Slots)-len(slotsToTransfer)]
		receiver.Slots = append(receiver.Slots, slotsToTransfer...)

		// Check if donor or receiver is now balanced
		if len(donor.Slots) <= idealSlots {
			donorIdx++
		}
		if len(receiver.Slots) >= idealSlots {
			receiverIdx++
		}
	}

	return plan
}

func displayRebalancePlan(plan []RebalancePlan, masters []MasterNode) {
	fmt.Println(styles.TitleStyle.Render("리밸런싱 계획"))

	// Show current distribution
	fmt.Println(styles.InfoStyle.Render("현재 슬롯 분배:"))
	for _, master := range masters {
		addr := master.Addr
		if strings.Contains(addr, "@") {
			addr = strings.Split(addr, "@")[0]
		}
		fmt.Printf("  %s: %s 슬롯\n",
			styles.HighlightStyle.Render(addr),
			styles.HighlightStyle.Render(strconv.Itoa(len(master.Slots))))
	}

	fmt.Println()
	fmt.Println(styles.InfoStyle.Render("이동 계획:"))
	totalSlots := 0
	for i, p := range plan {
		fromAddr := ""
		toAddr := ""
		for _, master := range masters {
			if master.ID == p.From {
				fromAddr = master.Addr
			}
			if master.ID == p.To {
				toAddr = master.Addr
			}
		}

		if strings.Contains(fromAddr, "@") {
			fromAddr = strings.Split(fromAddr, "@")[0]
		}
		if strings.Contains(toAddr, "@") {
			toAddr = strings.Split(toAddr, "@")[0]
		}

		fmt.Printf("  %d. %s → %s: %s 슬롯\n",
			i+1,
			styles.WarningStyle.Render(fromAddr),
			styles.SuccessStyle.Render(toAddr),
			styles.HighlightStyle.Render(strconv.Itoa(p.SlotCount)))
		totalSlots += p.SlotCount
	}

	fmt.Printf("\n총 이동할 슬롯: %s\n", styles.HighlightStyle.Render(strconv.Itoa(totalSlots)))
}

func executeRebalancePlan(ctx context.Context, client *redis.ClusterClient, plan []RebalancePlan, pipeline int) error {
	fmt.Println()
	fmt.Println(styles.InfoStyle.Render("3. 리밸런싱 실행 중..."))

	totalSlots := 0
	for _, p := range plan {
		totalSlots += p.SlotCount
	}

	processedSlots := 0

	for i, p := range plan {
		fmt.Printf("  %d/%d 단계: %d개 슬롯 이동 중... ",
			i+1, len(plan), p.SlotCount)

		startTime := time.Now()

		// Use the reshard logic to move slots
		if err := reshardSlots(ctx, client, p.From, p.To, p.Slots, pipeline); err != nil {
			// Provide more detailed error information
			fmt.Printf("\n    X 실패: %v\n", err)
			fmt.Printf("      부분 완료: %d/%d 슬롯 이동됨\n", processedSlots, totalSlots)
			fmt.Printf("     수동 복구가 필요할 수 있습니다. 'check' 명령으로 현재 상태를 확인하세요.\n")
			return fmt.Errorf("슬롯 이동 실패 (단계 %d): %w", i+1, err)
		}

		duration := time.Since(startTime)
		processedSlots += p.SlotCount

		progress := float64(processedSlots) / float64(totalSlots) * 100
		fmt.Printf("OK 완료 (%.1fs, 진행률: %.1f%%)\n", duration.Seconds(), progress)
	}

	return nil
}

// Reuse reshard logic for slot migration
func reshardSlots(ctx context.Context, client *redis.ClusterClient, fromID, toID string, slots []int, pipeline int) error {
	// Get source and target client connections
	sourceClient, err := getNodeClient(ctx, client, fromID)
	if err != nil {
		return fmt.Errorf("소스 노드 클라이언트 생성 실패: %w", err)
	}
	defer sourceClient.Close()

	targetClient, err := getNodeClient(ctx, client, toID)
	if err != nil {
		return fmt.Errorf("대상 노드 클라이언트 생성 실패: %w", err)
	}
	defer targetClient.Close()

	// Get target node address
	targetAddr, err := getNodeAddressFromCluster(ctx, client, toID)
	if err != nil {
		return fmt.Errorf("대상 노드 주소 조회 실패: %w", err)
	}

	parts := strings.Split(targetAddr, ":")
	if len(parts) != 2 {
		return fmt.Errorf("잘못된 노드 주소 형식: %s", targetAddr)
	}

	targetHost := parts[0]
	targetPort := parts[1]

	for _, slot := range slots {
		// Step 1: Set slot as migrating on source
		err := sourceClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "MIGRATING", toID).Err()
		if err != nil {
			return fmt.Errorf("MIGRATING 설정 실패 (슬롯 %d): %w", slot, err)
		}

		// Step 2: Set slot as importing on target
		err = targetClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "IMPORTING", fromID).Err()
		if err != nil {
			return fmt.Errorf("IMPORTING 설정 실패 (슬롯 %d): %w", slot, err)
		}

		// Step 3: Migrate all keys in this slot - repeat until slot is empty
		for {
			// Get keys in the slot (limited by pipeline size)
			keys := sourceClient.ClusterGetKeysInSlot(ctx, slot, pipeline)
			if keys.Err() != nil {
				return fmt.Errorf("슬롯 %d 키 조회 실패: %w", slot, keys.Err())
			}

			// If no keys left, migration is complete
			if len(keys.Val()) == 0 {
				break
			}

			// Migrate keys in batches for better performance
			if err := migrateKeysBatch(ctx, sourceClient, keys.Val(), targetHost, targetPort); err != nil {
				return fmt.Errorf("키 배치 마이그레이션 실패 (슬롯 %d): %w", slot, err)
			}
		}

		// Step 4: Set slot to stable state on both nodes
		err = sourceClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", toID).Err()
		if err != nil {
			return fmt.Errorf("소스 노드 슬롯 상태 설정 실패 (슬롯 %d): %w", slot, err)
		}

		err = targetClient.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", toID).Err()
		if err != nil {
			return fmt.Errorf("대상 노드 슬롯 상태 설정 실패 (슬롯 %d): %w", slot, err)
		}
	}

	return nil
}

func getNodeClient(ctx context.Context, client *redis.ClusterClient, nodeID string) (*redis.Client, error) {
	// Get node address
	addr, err := getNodeAddressFromCluster(ctx, client, nodeID)
	if err != nil {
		return nil, err
	}

	// Create individual client for this node
	user, password := config.GetAuth()
	nodeClient := redis.NewClient(&redis.Options{
		Addr:     addr,
		Username: user,
		Password: password,
	})

	// Test connection
	if err := nodeClient.Ping(ctx).Err(); err != nil {
		nodeClient.Close()
		return nil, fmt.Errorf("노드 연결 실패: %w", err)
	}

	return nodeClient, nil
}

func getNodeAddressFromCluster(ctx context.Context, client *redis.ClusterClient, nodeID string) (string, error) {
	result := client.ClusterNodes(ctx)
	if result.Err() != nil {
		return "", result.Err()
	}

	lines := strings.Split(result.Val(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == nodeID {
			addr := parts[1]
			if strings.Contains(addr, "@") {
				addr = strings.Split(addr, "@")[0]
			}
			return addr, nil
		}
	}

	return "", fmt.Errorf("노드 ID %s를 찾을 수 없습니다", nodeID)
}

// migrateKeysBatch migrates multiple keys efficiently using pipelining
func migrateKeysBatch(ctx context.Context, sourceClient *redis.Client, keys []string, targetHost, targetPort string) error {
	if len(keys) == 0 {
		return nil
	}

	user, password := config.GetAuth()

	// Use pipeline for better performance when migrating multiple keys
	pipeline := sourceClient.Pipeline()

	for _, key := range keys {
		var migrateCmd []any
		if user != "" {
			// AUTH2 format: MIGRATE host port key destination-db timeout [COPY | REPLACE] [AUTH2 username password]
			migrateCmd = []any{"MIGRATE", targetHost, targetPort, key, 0, 60000, "AUTH2", user, password}
		} else {
			// AUTH format: MIGRATE host port key destination-db timeout [COPY | REPLACE] [AUTH password]
			migrateCmd = []any{"MIGRATE", targetHost, targetPort, key, 0, 60000, "AUTH", password}
		}
		pipeline.Do(ctx, migrateCmd...)
	}

	// Execute all migrations in one batch
	results, err := pipeline.Exec(ctx)
	if err != nil {
		return fmt.Errorf("파이프라인 실행 실패: %w", err)
	}

	// Check individual results
	for i, result := range results {
		if result.Err() != nil {
			return fmt.Errorf("키 %s 마이그레이션 실패: %w", keys[i], result.Err())
		}
	}

	return nil
}
